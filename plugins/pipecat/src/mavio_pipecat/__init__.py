"""Voice sessions of the Mavio assistant on Pipecat.

A session is a Pipecat pipeline on a WebSocket: the client's microphone
comes in as 16 kHz PCM, its turns are recognized and taken to the server's
Ask, and the answer goes back as 24 kHz PCM, with the protocol's events as
JSON text messages (docs/assistant.md).
"""
