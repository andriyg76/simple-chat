package e2e

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

var javaListenRE = regexp.MustCompile(`simple-chat test-server listening on (https?://[^\s]+)`)

func repoJavaDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("caller")
	}
	// .../go/e2e/java_server.go -> .../java
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "java")), nil
}

func startJavaTestServer(t *testing.T) (baseURL, token string, stop func()) {
	t.Helper()
	if os.Getenv("SKIP_JAVA_E2E") == "1" {
		t.Skip("SKIP_JAVA_E2E=1")
	}
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not on PATH")
	}
	if _, err := exec.LookPath("mvn"); err != nil {
		t.Skip("mvn not on PATH")
	}
	javaDir, err := repoJavaDir()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pack := exec.CommandContext(ctx, "mvn", "-q", "-f", javaDir, "-pl", "test-server", "-am", "package", "-DskipTests")
	pack.Stdout = os.Stdout
	pack.Stderr = os.Stderr
	if err := pack.Run(); err != nil {
		t.Fatalf("mvn package: %v", err)
	}
	jar := filepath.Join(javaDir, "test-server", "target", "test-server-0.1.0-SNAPSHOT-exec.jar")
	if _, err := os.Stat(jar); err != nil {
		t.Fatalf("missing jar %s: %v", jar, err)
	}
	runCtx, runCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(runCtx, "java", "-jar", jar)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		runCancel()
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		runCancel()
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	deadline := time.Now().Add(90 * time.Second)
	var base, tok string
	for scanner.Scan() && time.Now().Before(deadline) {
		line := scanner.Text()
		if m := javaListenRE.FindStringSubmatch(line); len(m) == 2 {
			base = strings.TrimRight(m[1], "/")
			tok = "dev"
			break
		}
	}
	if base == "" {
		runCancel()
		_ = cmd.Wait()
		t.Fatal("timed out waiting for Java test-server listen line")
	}
	return base, tok, func() {
		runCancel()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}
