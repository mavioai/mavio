"""A client of the host API."""

import os
from collections.abc import Mapping

import httpx

from mavio_plugin.serve import ENV_HOST_ADDRESS, ENV_HOST_SOCKET, ENV_TOKEN, HOST_URL, loopback


def host_client(
    headers: Mapping[str, str] | None = None, timeout: httpx.Timeout | None = None
) -> httpx.AsyncClient:
    """An HTTP client of the host API, whose relative URLs are the host
    API's and whose requests carry the plugin's token and headers; it fails
    outside a Mavio host.

    Requests go over the host socket, or on Windows to the host's loopback
    address. Connect's procedures are POSTed to "/<service>/<method>".
    """
    socket = os.environ.get(ENV_HOST_SOCKET, "")
    address = os.environ.get(ENV_HOST_ADDRESS, "")
    if loopback() and address:
        transport, base_url = httpx.AsyncHTTPTransport(), f"http://{address}"
    elif socket:
        transport, base_url = httpx.AsyncHTTPTransport(uds=socket), HOST_URL
    else:
        raise RuntimeError("the host API is not available outside a Mavio host")
    return httpx.AsyncClient(
        base_url=base_url,
        transport=transport,
        headers={"Authorization": "Bearer " + os.environ.get(ENV_TOKEN, ""), **(headers or {})},
        # Streams, such as Ask's, wait for their messages as long as they
        # take.
        timeout=timeout or httpx.Timeout(10, read=None),
    )


_REPORT_TASK_PROGRESS = "/mavio.plugin.v1.HostService/ReportTaskProgress"


async def report_task_progress(host: httpx.AsyncClient, task_id: str, progress: float) -> bool:
    """Tells the host the fraction done, 0 to 1, of the plugin's task
    running, which it shows as its job's progress; the host ignores a
    report of a task not running. It returns whether the host answered,
    for a plugin to log, never failing the run."""
    progress = min(max(progress, 0.0), 1.0)
    try:
        r = await host.post(
            _REPORT_TASK_PROGRESS,
            json={"taskId": task_id, "progress": progress},
            headers={"Content-Type": "application/json"},
        )
    except httpx.HTTPError:
        return False
    return r.status_code == 200
