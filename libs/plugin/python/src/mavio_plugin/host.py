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
