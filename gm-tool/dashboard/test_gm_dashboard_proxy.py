"""Offline proxy regressions: no database, listener, or running GM required."""
import io
import json
import unittest
import urllib.error
from unittest.mock import patch

import gm_dashboard_proxy as proxy


class Response:
    status = 200
    headers = {"Content-Type": "application/json; charset=utf-8"}

    def __init__(self, body):
        self.body = body

    def __enter__(self):
        return self

    def __exit__(self, *_):
        pass

    def read(self):
        return self.body


class ProxyTest(unittest.TestCase):
    def handler(self, path, body=b""):
        handler = object.__new__(proxy.Handler)
        handler.path = path
        handler.headers = {"Content-Length": str(len(body)), "Content-Type": "application/json"}
        handler.rfile = io.BytesIO(body)
        handler.wfile = io.BytesIO()
        handler.send_response = lambda status: setattr(handler, "status", status)
        handler.send_header = lambda *_: None
        handler.end_headers = lambda: None
        return handler

    def test_post_token_refresh_preserves_body_and_mail_response(self):
        raw = json.dumps({"title": "中文|O'Brien\n第二行", "amount": 1}).encode("utf-8")
        handler = self.handler("/api/mail/send?token=untrusted", raw)
        receipt = b'{"ok":true,"mail_id":42}'
        calls = []

        def urlopen(request, **_):
            calls.append(request)
            if len(calls) == 1:
                raise urllib.error.HTTPError(request.full_url, 401, "expired", {}, io.BytesIO(b"unauthorized"))
            return Response(receipt)

        def refresh():
            proxy.TOKEN = "new-token"
            return True

        with patch.object(proxy, "TOKEN", "old-token"), patch.object(proxy, "fetch_token", refresh), patch.object(proxy.urllib.request, "urlopen", urlopen):
            handler.do_POST()
        self.assertEqual(handler.status, 200)
        self.assertEqual(handler.wfile.getvalue(), receipt)
        self.assertEqual(len(calls), 2)
        self.assertEqual([r.data for r in calls], [raw, raw])
        self.assertTrue(calls[0].full_url.endswith("token=old-token"))
        self.assertTrue(calls[1].full_url.endswith("token=new-token"))

    def test_vault_failure_is_forwarded_without_retry(self):
        handler = self.handler("/api/vault/send", b'{"amount":1}')
        body = b'{"ok":false,"error":"vault full"}'
        error = urllib.error.HTTPError("http://backend", 400, "bad request", {"Content-Type": "application/json"}, io.BytesIO(body))
        with patch.object(proxy, "TOKEN", "token"), patch.object(proxy.urllib.request, "urlopen", side_effect=error) as request:
            handler.do_POST()
        self.assertEqual(handler.status, 400)
        self.assertEqual(handler.wfile.getvalue(), body)
        self.assertEqual(request.call_count, 1)

    def test_mail_list_uses_backend_and_preserves_filters(self):
        handler = self.handler("/api/mail/list?account=7&status=read")
        with patch.object(proxy, "TOKEN", "token"), patch.object(proxy.urllib.request, "urlopen", return_value=Response(b'{"ok":true,"count":0,"mails":[]}')) as request:
            handler.do_GET()
        self.assertIn("/api/mail/list?account=7&status=read&token=token", request.call_args.args[0].full_url)
        self.assertEqual(json.loads(handler.wfile.getvalue())["mails"], [])


if __name__ == "__main__":
    unittest.main()
