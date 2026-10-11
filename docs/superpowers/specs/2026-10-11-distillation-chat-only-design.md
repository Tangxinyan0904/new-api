# Distillation Detection Counts Only Chat Requests

Date: 2026-10-11

## Problem

Distillation detection counted every validated non-stream request that went
through `controller.Relay()`, including embeddings, rerank, images, audio and
moderations. A user batch-calling embeddings could be detected as distilling,
receive a temporary limit, and eventually have all non-stream requests
permanently blocked.

## Decision

Detection, temporary limits and the permanent ban apply only to non-stream
conversational text generation:

| Endpoint | Relay format / mode |
| --- | --- |
| `/v1/chat/completions` | OpenAI, chat completions |
| `/v1/completions` | OpenAI, completions |
| `/v1/responses` | OpenAI Responses |
| `/v1/messages` | Claude |
| `/v1beta/models/*:generateContent`, `/v1/models/*:generateContent` | Gemini |

Everything else is neither counted nor limited, the same way streaming
requests already are: embeddings (including Gemini `embedContent` and
`batchEmbedContents`), rerank, images, audio, moderations, alpha search,
`/v1/responses/compact` and realtime sessions.

Context compaction is excluded because coding agents call it non-streaming
and often during long sessions, and its output is a summary of the user's
own context. Realtime is a streaming session that previously counted only
because its relay info does not set `IsStream`.

## Implementation

`service.CheckDistillationRateLimit` returns early unless
`isDistillationTrackedRequest` matches the request's relay format, relay mode
and, for Gemini, the `:generateContent` action. The counter, penalty storage
and settings are unchanged.

The admin setting description and the ban error message now say "chat
requests". The description also says "per clock minute", which matches the
fixed-minute counter (it previously said "rolling one-minute window").

## Testing

`TestDistillationRateLimitOnlyChecksConversationRequests` runs every
endpoint above through the public check with an unavailable runtime store:
tracked requests reach the store and fail, while exempt requests return
without touching any distillation state.
