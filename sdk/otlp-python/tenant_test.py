"""Tenant header test: X-Tenant-Id is attached on every ingest."""
import json
import threading
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer

from demo_dog import Client


class H(BaseHTTPRequestHandler):
    headers_seen = {}

    def do_POST(self):  # noqa: N802
        H.headers_seen = {k.lower(): v for k, v in self.headers.items()}
        self.send_response(202)
        self.end_headers()
        self.wfile.write(b"{}")

    def log_message(self, *a, **kw):  # noqa: A003
        return


def main():
    srv = HTTPServer(("127.0.0.1", 0), H)
    port = srv.server_address[1]
    t = threading.Thread(target=srv.serve_forever, daemon=True)
    t.start()
    c = Client(base_url="http://127.0.0.1:" + str(port), api_key="x", tenant="acme", flush_interval=0)
    c.counter("m", 1)
    c.flush()
    srv.shutdown()
    assert H.headers_seen.get("x-tenant-id") == "acme", H.headers_seen
    print("OK tenant")


if __name__ == "__main__":
    main()
