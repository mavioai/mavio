"""The guest side of Mavio's process plugins written in Python, as
guest/process is for Go and @mavio/plugin for TypeScript.

serve runs the plugin's Connect services, its HTTP routes and the paths it
mounts as one ASGI application, served by Hypercorn on the socket the host
gives (h2c and HTTP/1.1 with WebSockets; on Windows on loopback TCP). It
checks the host's token, prints the handshake line, answers PluginService
and returns after Shutdown, a termination signal, or the end of stdin when
the host makes it a lifeline. host_client reaches the host API, and
report_task_progress tells it how far a task run is.
"""

from mavio_plugin.host import host_client, report_task_progress
from mavio_plugin.serve import (
    ENV_DATA,
    ENV_DEVICE_ID,
    ENV_HOST_ADDRESS,
    ENV_HOST_OS,
    ENV_HOST_SOCKET,
    ENV_LIFELINE,
    ENV_SOCKET,
    ENV_TOKEN,
    HOST_URL,
    HTTP_PREFIX,
    PROTOCOL_VERSION,
    ASGIApp,
    data_dir,
    device_id,
    host_os,
    load_manifest,
    loopback,
    serve,
)

__all__ = [
    "ENV_DATA",
    "ENV_DEVICE_ID",
    "ENV_HOST_ADDRESS",
    "ENV_HOST_OS",
    "ENV_HOST_SOCKET",
    "ENV_LIFELINE",
    "ENV_SOCKET",
    "ENV_TOKEN",
    "HOST_URL",
    "HTTP_PREFIX",
    "PROTOCOL_VERSION",
    "ASGIApp",
    "data_dir",
    "device_id",
    "host_client",
    "host_os",
    "load_manifest",
    "loopback",
    "report_task_progress",
    "serve",
]
