# WebSocket Event Streaming API

This document describes the WebSocket API for real-time event streaming from Council of Legends sessions.

## Connection

### Endpoint

```
ws://localhost:8080/api/v1/events/{sessionId}
```

Where `{sessionId}` is either a debate ID or team session ID.

### Authentication

Include the authentication token as a query parameter or in the first message:

```
ws://localhost:8080/api/v1/events/{sessionId}?token={jwt_token}
```

Or send as the first message after connection:

```json
{
  "type": "auth",
  "token": "your-jwt-token"
}
```

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `token` | string | JWT authentication token |
| `filter` | string[] | Event types to receive (comma-separated) |

Example with filter:
```
ws://localhost:8080/api/v1/events/{sessionId}?filter=phase_changed,task_completed
```

## Message Format

All messages follow this envelope format:

```json
{
  "type": "event",
  "timestamp": "2026-01-28T15:30:00Z",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "sessionType": "debate|team",
  "event": {
    // Event-specific payload
  }
}
```

## Debate Events

### debate.started

Emitted when a debate session begins.

```json
{
  "type": "debate.started",
  "topic": "Should AI be regulated?",
  "mode": "adversarial",
  "members": ["claude", "gpt-4o", "gemini"],
  "totalRounds": 3
}
```

### debate.round_started

Emitted when a new round begins.

```json
{
  "type": "debate.round_started",
  "round": 1,
  "phase": "opening"
}
```

### debate.response

Emitted when an AI submits a response. If streaming is enabled, this is sent as chunks.

```json
{
  "type": "debate.response",
  "aiId": "claude",
  "aiName": "Claude",
  "round": 1,
  "phase": "opening",
  "content": "I believe that...",
  "isChunk": false,
  "done": true
}
```

For streaming responses:

```json
{
  "type": "debate.response",
  "aiId": "claude",
  "aiName": "Claude",
  "round": 1,
  "phase": "opening",
  "content": "I believe",
  "isChunk": true,
  "done": false
}
```

### debate.round_completed

Emitted when a round finishes.

```json
{
  "type": "debate.round_completed",
  "round": 1,
  "phase": "opening"
}
```

### debate.synthesis

Emitted when an AI provides their synthesis.

```json
{
  "type": "debate.synthesis",
  "aiId": "claude",
  "aiName": "Claude",
  "content": "In conclusion..."
}
```

### debate.final_verdict

Emitted when the council reaches a final verdict.

```json
{
  "type": "debate.final_verdict",
  "content": "The council has determined that..."
}
```

### debate.completed

Emitted when the debate session ends.

```json
{
  "type": "debate.completed",
  "duration": "15m30s",
  "transcriptPath": "/debates/2026-01-28-ai-regulation.md"
}
```

### debate.error

Emitted when an error occurs.

```json
{
  "type": "debate.error",
  "message": "Failed to invoke claude: API timeout",
  "recoverable": true
}
```

## Team Events

### team.phase_changed

Emitted when the team session transitions to a new phase.

```json
{
  "type": "team.phase_changed",
  "phase": "planning",
  "previousPhase": "analysis"
}
```

### team.pm_selected

Emitted when a project manager is selected.

```json
{
  "type": "team.pm_selected",
  "pmId": "claude",
  "pmName": "Claude"
}
```

### team.pm_decision

Emitted when the PM makes a planning decision.

```json
{
  "type": "team.pm_decision",
  "plan": {
    "summary": "Build REST API with authentication",
    "steps": [
      {
        "id": "step-1",
        "description": "Design API schema",
        "assignedTo": "claude",
        "status": "pending"
      }
    ]
  },
  "mode": "divide_conquer"
}
```

### team.task_created

Emitted when a new task is created.

```json
{
  "type": "team.task_created",
  "taskId": "step-1",
  "description": "Design API schema",
  "assignedTo": "claude"
}
```

### team.task_started

Emitted when an AI starts working on a task.

```json
{
  "type": "team.task_started",
  "taskId": "step-1",
  "actor": "claude"
}
```

### team.task_progress

Emitted for task progress updates.

```json
{
  "type": "team.task_progress",
  "taskId": "step-1",
  "actor": "claude",
  "message": "Completed endpoint design, working on schemas...",
  "percentComplete": 50
}
```

### team.task_completed

Emitted when a task is finished.

```json
{
  "type": "team.task_completed",
  "taskId": "step-1",
  "actor": "claude",
  "artifacts": ["api-schema.yaml"]
}
```

### team.artifact_created

Emitted when an artifact is generated.

```json
{
  "type": "team.artifact_created",
  "name": "handler.go",
  "type": "code",
  "createdBy": "claude",
  "size": 2048,
  "description": "HTTP request handlers"
}
```

### team.checkpoint_created

Emitted when a checkpoint requires approval.

```json
{
  "type": "team.checkpoint_created",
  "checkpointId": "cp-1",
  "checkpointType": "plan_approval",
  "description": "Approve the project plan before execution",
  "blocking": true
}
```

### team.checkpoint_approved

Emitted when a checkpoint is approved.

```json
{
  "type": "team.checkpoint_approved",
  "checkpointId": "cp-1",
  "approvedBy": "user",
  "notes": "Looks good, proceed"
}
```

### team.checkpoint_rejected

Emitted when a checkpoint is rejected.

```json
{
  "type": "team.checkpoint_rejected",
  "checkpointId": "cp-1",
  "rejectedBy": "user",
  "reason": "Need to add error handling step"
}
```

### team.user_task_blocking

Emitted when user action is required.

```json
{
  "type": "team.user_task_blocking",
  "taskId": "user-1",
  "description": "Please provide database credentials",
  "action": "provide_input"
}
```

### team.session_complete

Emitted when the team session finishes.

```json
{
  "type": "team.session_complete",
  "duration": "45m",
  "artifactCount": 5,
  "projectDir": "/projects/rest-api-2026-01-28"
}
```

### team.error

Emitted when an error occurs.

```json
{
  "type": "team.error",
  "message": "Failed to invoke gemini: rate limited",
  "taskId": "step-3",
  "recoverable": true
}
```

## Client Messages

Clients can send messages to control the session.

### Ping/Pong

Keep the connection alive:

```json
{
  "type": "ping"
}
```

Response:

```json
{
  "type": "pong",
  "timestamp": "2026-01-28T15:30:00Z"
}
```

### Subscribe/Unsubscribe

Dynamically filter events:

```json
{
  "type": "subscribe",
  "events": ["team.task_completed", "team.artifact_created"]
}
```

```json
{
  "type": "unsubscribe",
  "events": ["team.task_progress"]
}
```

### Checkpoint Response

Approve or reject a checkpoint:

```json
{
  "type": "checkpoint_approve",
  "checkpointId": "cp-1",
  "notes": "Approved via WebSocket"
}
```

```json
{
  "type": "checkpoint_reject",
  "checkpointId": "cp-1",
  "reason": "Missing error handling"
}
```

### User Input Response

Respond to a blocking user task:

```json
{
  "type": "user_input",
  "taskId": "user-1",
  "input": "postgres://user:pass@localhost/db"
}
```

## Error Handling

### Connection Errors

If the WebSocket connection fails, clients receive:

```json
{
  "type": "error",
  "code": "SESSION_NOT_FOUND",
  "message": "Session 550e8400-... does not exist"
}
```

Error codes:
- `AUTH_REQUIRED`: Authentication token missing
- `AUTH_INVALID`: Authentication token invalid/expired
- `SESSION_NOT_FOUND`: Session ID does not exist
- `SESSION_ENDED`: Session has already completed
- `RATE_LIMITED`: Too many connections

### Reconnection

Clients should implement exponential backoff for reconnection:

1. Wait 1 second, attempt reconnect
2. Wait 2 seconds, attempt reconnect
3. Wait 4 seconds, attempt reconnect
4. Maximum 30 second delay

Upon reconnection, send a `replay` message to catch up on missed events:

```json
{
  "type": "replay",
  "since": "2026-01-28T15:30:00Z"
}
```

The server will replay all events since that timestamp.

## Example Client (JavaScript)

```javascript
const ws = new WebSocket(
  `ws://localhost:8080/api/v1/events/${sessionId}?token=${token}`
);

ws.onopen = () => {
  console.log('Connected to session');

  // Subscribe to specific events
  ws.send(JSON.stringify({
    type: 'subscribe',
    events: ['team.task_completed', 'team.artifact_created']
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  switch (msg.event?.type) {
    case 'team.checkpoint_created':
      if (msg.event.blocking) {
        // Show approval UI
        showCheckpointDialog(msg.event);
      }
      break;

    case 'team.artifact_created':
      // Update artifact list
      addArtifact(msg.event);
      break;

    case 'debate.response':
      if (msg.event.isChunk) {
        // Append streaming content
        appendContent(msg.event.aiId, msg.event.content);
      } else {
        // Complete response
        setResponse(msg.event.aiId, msg.event.content);
      }
      break;
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = (event) => {
  if (!event.wasClean) {
    // Implement reconnection with backoff
    scheduleReconnect();
  }
};

function approveCheckpoint(checkpointId, notes) {
  ws.send(JSON.stringify({
    type: 'checkpoint_approve',
    checkpointId,
    notes
  }));
}
```
