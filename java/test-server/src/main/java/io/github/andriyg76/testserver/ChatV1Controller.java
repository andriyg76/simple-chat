package io.github.andriyg76.testserver;

import io.github.andriyg76.core.ChatException;
import io.github.andriyg76.protocol.ChatErrorBody;
import io.github.andriyg76.protocol.ChatErrorEnvelope;
import io.github.andriyg76.protocol.ConversationListResponse;
import io.github.andriyg76.protocol.ConversationSession;
import io.github.andriyg76.protocol.EnrollmentExchangeRequest;
import io.github.andriyg76.protocol.EnrollmentExchangeResponse;
import io.github.andriyg76.protocol.AddMemberRequest;
import io.github.andriyg76.protocol.DeviceHeartbeatRequest;
import io.github.andriyg76.protocol.DevicePutRequest;
import io.github.andriyg76.protocol.DeviceRecord;
import io.github.andriyg76.protocol.ConversationMembersResponse;
import io.github.andriyg76.protocol.PostedMessage;
import io.github.andriyg76.protocol.SendMessageRequest;
import io.github.andriyg76.protocol.UpdateConversationRequest;
import io.github.andriyg76.server.ChatV1Service;
import jakarta.servlet.http.HttpServletRequest;
import java.io.IOException;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.ResponseStatus;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;

@RestController
public class ChatV1Controller {

    private final ChatV1Service chat;

    public ChatV1Controller(ChatV1Service chat) {
        this.chat = chat;
    }

    @GetMapping("/health")
    public String health() {
        return "ok";
    }

    @PostMapping("/chat/v1/enrollments/exchange")
    public EnrollmentExchangeResponse exchange(@RequestBody EnrollmentExchangeRequest body) {
        return chat.exchangeEnrollment(body.code());
    }

    @GetMapping("/chat/v1/profile")
    public io.github.andriyg76.protocol.ProfileResponse profile(
            @RequestHeader(value = "Authorization", required = false) String authorization) {
        return chat.currentProfile(bearer(authorization));
    }

    @GetMapping("/chat/v1/conversations")
    public ConversationListResponse list(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @RequestParam(defaultValue = "false") boolean archived) {
        return chat.listConversations(bearer(authorization), archived);
    }

    @PostMapping("/chat/v1/conversations")
    public ConversationSession create(@RequestHeader(value = "Authorization", required = false) String authorization) {
        return chat.createConversation(bearer(authorization));
    }

    @PostMapping("/chat/v1/conversations/{code}/messages/plain")
    public Object sendPlain(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String code,
            @RequestBody SendMessageRequest body) {
        return chat.sendPlain(bearer(authorization), code, body);
    }

    @PostMapping("/chat/v1/conversations/{code}/messages/async")
    @ResponseStatus(HttpStatus.ACCEPTED)
    public PostedMessage sendAsync(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String code,
            @RequestBody SendMessageRequest body) {
        return chat.sendAsync(bearer(authorization), code, body);
    }

    @PatchMapping("/chat/v1/conversations/{code}")
    public ConversationSession updateTitle(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String code,
            @RequestBody UpdateConversationRequest body) {
        return chat.updateTitle(bearer(authorization), code, body);
    }

    @GetMapping("/chat/v1/conversations/{code}/members")
    public ConversationMembersResponse listMembers(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String code) {
        return chat.listMembers(bearer(authorization), code);
    }

    @PostMapping("/chat/v1/conversations/{code}/members")
    @ResponseStatus(HttpStatus.CREATED)
    public void addMember(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String code,
            @RequestBody AddMemberRequest body) {
        chat.addMember(bearer(authorization), code, body);
    }

    @DeleteMapping("/chat/v1/conversations/{code}/members/{profileId}")
    @ResponseStatus(HttpStatus.NO_CONTENT)
    public void removeMember(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String code,
            @PathVariable String profileId) {
        chat.removeMember(bearer(authorization), code, profileId);
    }

    @PutMapping("/chat/v1/devices/{deviceId}")
    public DeviceRecord upsertDevice(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String deviceId,
            @RequestBody DevicePutRequest body) {
        return chat.upsertDevice(bearer(authorization), deviceId, body);
    }

    @DeleteMapping("/chat/v1/devices/{deviceId}")
    @ResponseStatus(HttpStatus.NO_CONTENT)
    public void deleteDevice(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String deviceId) {
        chat.deleteDevice(bearer(authorization), deviceId);
    }

    @PostMapping("/chat/v1/devices/{deviceId}/heartbeat")
    @ResponseStatus(HttpStatus.NO_CONTENT)
    public void heartbeatDevice(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @PathVariable String deviceId,
            @RequestBody(required = false) DeviceHeartbeatRequest body) {
        chat.heartbeatDevice(bearer(authorization), deviceId, body);
    }

    @GetMapping("/chat/v1/inbox/cursor")
    public io.github.andriyg76.protocol.InboxCursorResponse inboxCursor(
            @RequestHeader(value = "Authorization", required = false) String authorization) {
        return chat.inboxCursor(bearer(authorization));
    }

    @GetMapping("/chat/v1/inbox/feed")
    public io.github.andriyg76.protocol.InboxFeedResponse inboxFeed(
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @RequestParam(defaultValue = "0") long cursor,
            @RequestParam(defaultValue = "1") int limit,
            @RequestParam(defaultValue = "30") int wait) {
        return chat.pollInbox(bearer(authorization), cursor, limit, wait);
    }

    @GetMapping("/chat/v1/inbox/events")
    public ResponseEntity<StreamingResponseBody> inboxEvents(
            HttpServletRequest request,
            @RequestHeader(value = "Authorization", required = false) String authorization,
            @RequestParam(required = false) Long cursor) {
        String token = bearer(authorization);
        long after = resolveInboxCursor(request, cursor);
        var participant = chat.requireParticipant(token);
        try {
            chat.core().inboxNext(participant.id(), after, 1, 0L);
        } catch (ChatException e) {
            throw e;
        }
        StreamingResponseBody body = outputStream -> {
            try {
                io.github.andriyg76.server.ChatInboxSseSupport.stream(chat, token, after, outputStream);
            } catch (ChatException e) {
                if (Thread.currentThread().isInterrupted()) {
                    return;
                }
                if ("inbox_cursor_expired".equals(e.errorCode())) {
                    io.github.andriyg76.server.ChatInboxSseSupport.writeInboxCursorExpired(outputStream);
                    return;
                }
                throw e;
            } catch (IOException e) {
                if (Thread.currentThread().isInterrupted()) {
                    return;
                }
                throw e;
            }
        };
        return ResponseEntity.ok()
                .contentType(MediaType.TEXT_EVENT_STREAM)
                .header("Cache-Control", "no-cache")
                .header("X-Accel-Buffering", "no")
                .body(body);
    }

    @ExceptionHandler(ChatException.class)
    public ResponseEntity<ChatErrorEnvelope> chatError(ChatException e) {
        return ResponseEntity.status(e.httpStatus())
                .body(new ChatErrorEnvelope(new ChatErrorBody(e.errorCode(), e.getMessage())));
    }

    private static long resolveInboxCursor(HttpServletRequest request, Long queryCursor) {
        String lastEventId = request.getHeader("Last-Event-ID");
        if (lastEventId != null && !lastEventId.isEmpty()) {
            try {
                return Long.parseLong(lastEventId);
            } catch (NumberFormatException e) {
                throw ChatException.invalidCursor("invalid cursor");
            }
        }
        if (queryCursor != null) {
            return queryCursor;
        }
        return 0L;
    }

    private static String bearer(String authorization) {
        if (authorization == null) {
            throw ChatException.unauthorized("bearer token is required");
        }
        if (authorization.startsWith("Bearer ")) {
            return authorization.substring("Bearer ".length()).trim();
        }
        throw ChatException.unauthorized("bearer token is required");
    }
}
