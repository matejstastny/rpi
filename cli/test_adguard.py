"""the reconcile logic, against a stand-in for adguard's api.

the real thing needs a live box and its web password, so the endpoints pi
actually uses are reimplemented here. what this pins down is the part worth
pinning down: that a plan only ever touches names inside the zone, that a
changed address becomes a delete plus an add rather than two live answers, and
that applying twice is a no-op.
"""

from __future__ import annotations

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from . import adguard


class FakeAdGuard(BaseHTTPRequestHandler):
    rewrites: list[dict[str, str]] = []
    upstream: list[str] = []
    bootstrap: list[str] = []
    calls: list[str] = []

    def log_message(self, *args):
        pass

    def _reply(self, body):
        raw = json.dumps(body).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        type(self).calls.append(f"GET {self.path}")
        if self.path == "/control/rewrite/list":
            return self._reply(type(self).rewrites)
        if self.path == "/control/dns_info":
            return self._reply({"upstream_dns": type(self).upstream})
        self.send_error(404)

    def do_POST(self):
        type(self).calls.append(f"POST {self.path}")
        length = int(self.headers.get("Content-Length") or 0)
        body = json.loads(self.rfile.read(length) or b"{}")
        cls = type(self)

        if self.path == "/control/rewrite/add":
            cls.rewrites.append({"domain": body["domain"], "answer": body["answer"]})
            return self._reply({})
        if self.path == "/control/rewrite/delete":
            cls.rewrites = [
                r
                for r in cls.rewrites
                if not (r["domain"] == body["domain"] and r["answer"] == body["answer"])
            ]
            return self._reply({})
        if self.path == "/control/dns_config":
            cls.upstream = body["upstream_dns"]
            cls.bootstrap = body["bootstrap_dns"]
            return self._reply({})
        self.send_error(404)


class ReconcileTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = ThreadingHTTPServer(("127.0.0.1", 0), FakeAdGuard)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.port = cls.server.server_address[1]

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def setUp(self):
        FakeAdGuard.rewrites = []
        FakeAdGuard.upstream = []
        FakeAdGuard.bootstrap = []
        FakeAdGuard.calls = []
        self.client = adguard.Client(host="127.0.0.1", user="elara", password="x", port=self.port)
        self.want = adguard.desired()

    def test_empty_instance_gets_every_name(self):
        plan = adguard.plan(self.client, "thebe")
        self.assertEqual(len(plan.add), len(self.want))
        self.assertEqual(plan.remove, [])
        self.assertTrue(plan.upstream_changes)

        adguard.apply(self.client, plan)
        live = {r["domain"]: r["answer"] for r in FakeAdGuard.rewrites}
        self.assertEqual(live, {d: a for d, (a, _) in self.want.items()})

    def test_apply_is_idempotent(self):
        adguard.apply(self.client, adguard.plan(self.client, "thebe"))
        again = adguard.plan(self.client, "thebe")
        self.assertTrue(again.empty, f"second pass still wanted {again.add}{again.remove}")
        self.assertEqual(len(again.keep), len(self.want))

    def test_foreign_names_are_left_alone(self):
        FakeAdGuard.rewrites = [{"domain": "nas.lan", "answer": "192.168.250.9"}]
        plan = adguard.plan(self.client, "leda")
        self.assertEqual(plan.foreign, 1)
        self.assertNotIn("nas.lan", [d for d, _ in plan.remove])

        adguard.apply(self.client, plan)
        self.assertIn({"domain": "nas.lan", "answer": "192.168.250.9"}, FakeAdGuard.rewrites)

    def test_changed_address_replaces_rather_than_duplicates(self):
        domain, (answer, _) = next(iter(sorted(self.want.items())))
        FakeAdGuard.rewrites = [{"domain": domain, "answer": "10.0.0.1"}]

        plan = adguard.plan(self.client, "leda")
        self.assertIn((domain, "10.0.0.1"), plan.remove)
        self.assertIn(domain, [d for d, _, _ in plan.add])

        adguard.apply(self.client, plan)
        answers = [r["answer"] for r in FakeAdGuard.rewrites if r["domain"] == domain]
        self.assertEqual(answers, [answer], "a name must have exactly one answer")

    def test_stale_zone_name_is_removed(self):
        FakeAdGuard.rewrites = [{"domain": "old.elara.boo", "answer": "100.1.2.3"}]
        plan = adguard.plan(self.client, "leda")
        self.assertIn(("old.elara.boo", "100.1.2.3"), plan.remove)

        adguard.apply(self.client, plan)
        self.assertNotIn("old.elara.boo", [r["domain"] for r in FakeAdGuard.rewrites])

    def test_upstream_is_per_host(self):
        adguard.apply(self.client, adguard.plan(self.client, "thebe"))
        self.assertEqual(FakeAdGuard.upstream, ["127.0.0.1:5335"])

        FakeAdGuard.upstream = []
        adguard.apply(self.client, adguard.plan(self.client, "leda"))
        self.assertEqual(FakeAdGuard.upstream, ["https://dns10.quad9.net/dns-query"])
        self.assertTrue(FakeAdGuard.bootstrap, "bootstrap must be set alongside")


if __name__ == "__main__":
    unittest.main()
