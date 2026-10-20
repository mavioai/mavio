import json

import httpx

from mavio_plugin import report_task_progress


async def test_report_task_progress():
    got: list[tuple[str, object]] = []

    def serve(request: httpx.Request) -> httpx.Response:
        got.append((request.url.path, json.loads(request.content)))
        return httpx.Response(200, json={})

    host = httpx.AsyncClient(base_url="http://host", transport=httpx.MockTransport(serve))
    assert await report_task_progress(host, "scan", 1.5)
    want = [
        ("/mavio.plugin.v1.HostService/ReportTaskProgress", {"taskId": "scan", "progress": 1.0})
    ]
    assert got == want, f"got = {got}, want = {want}"


async def test_report_task_progress_without_the_host():
    def fail(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused", request=request)

    host = httpx.AsyncClient(base_url="http://host", transport=httpx.MockTransport(fail))
    assert not await report_task_progress(host, "scan", 0.5)
