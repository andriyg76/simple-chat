package io.github.andriyg76.client;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

class ChatHttpClientJsonTest {

    @Test
    void fieldDecodesEscapesAndToleratesWhitespace() {
        String json = "{\n  \"profile\" : {\n    \"code\" : \"u1\",\n    \"displayName\": \"A \\u0026 \\\"B\\\"\"\n  }\n}";

        assertEquals("u1", ChatHttpClient.Json.field(json, "code"));
        assertEquals("A & \"B\"", ChatHttpClient.Json.field(json, "displayName"));
    }
}
