"""Isolated UI check. Uses demo data and intercepted APIs; never contacts a live panel."""
import asyncio
import json
import sys
from pathlib import Path
from playwright.async_api import async_playwright

OUT = Path(__file__).parent
BASE = "http://127.0.0.1:14531"

async def main():
    behavior_only = "--behavior-only" in sys.argv
    report = json.loads((OUT / "browser-results.json").read_text(encoding="utf-8")) if behavior_only else {"screens": [], "errors": [], "behaviors": {}}
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="msedge", headless=True)
        ctx = await browser.new_context(viewport={"width": 1440, "height": 1000})
        page = await ctx.new_page()
        page.on("pageerror", lambda error: report["errors"].append(str(error)))
        cases = [("machines", "/machines"), ("nodes", "/nodes"), ("node-editor", "/nodes/nod_1042/edit"), ("node-history", "/nodes/nod_1042/history"), ("user-editor", "/users/usr_1001/edit"), ("plan-editor", "/plans/pln_standard/edit"), ("access-group", "/access-groups/grp_standard/edit"), ("settings", "/settings"), ("access-ips", "/access-ips"), ("audit", "/audit")]
        async def demo_open(route, width, theme):
            await page.set_viewport_size({"width": width, "height": 1000 if width > 400 else 844})
            await page.add_init_script(f"localStorage.setItem('cpanel.theme', '{theme}')")
            await page.goto(BASE + route + "?demo=1")
            await page.get_by_role("button", name="登录后台", exact=True).click()
            await page.locator(".admin-console h1").wait_for()
            await page.wait_for_timeout(450)
        for width, theme in ([] if behavior_only else [(1440, "light"), (390, "light"), (1024, "light"), (1440, "dark"), (390, "dark")]):
            targets = cases if width != 1024 and theme == "light" else [cases[i] for i in [0, 2, 3]]
            for name, route in targets:
                await demo_open(route, width, theme)
                filename = f"{name}-{width}-{theme}.png"
                overflow = await page.evaluate("document.documentElement.scrollWidth > innerWidth + 1")
                report["screens"].append({"file": filename, "overflow": overflow, "heading": await page.locator("h1").inner_text()})
                await page.screenshot(path=str(OUT / filename), full_page=False)
        (OUT / "browser-results.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
        # Demo history is navigable and restoration does not publish.
        await demo_open("/nodes/nod_1042/history", 1440, "light")
        await page.get_by_role("button", name="恢复为草稿", exact=True).click()
        await page.get_by_role("button", name="恢复并编辑", exact=True).click()
        await page.wait_for_url("**/nodes/nod_1042/edit?**")
        restored = await page.evaluate("async () => (await import('/src/mock.ts')).demoNodes[0]")
        assert restored["has_draft"] and restored["current_revision"] == 31 and restored["applied_revision"] == 31
        report["behaviors"]["history_restores_draft_without_publish"] = True
        fixtures = await page.evaluate("async () => { const m = await import('/src/mock.ts'); return Object.fromEntries(Object.entries(m).filter(([key]) => key.startsWith('demo') && typeof m[key] !== 'function')); }")
        await ctx.close()
        # Live auth/API paths with a simulated expired session, no actual server calls.
        live = await browser.new_context(viewport={"width": 1440, "height": 1000})
        page = await live.new_page()
        page.on("pageerror", lambda error: report["errors"].append(str(error)))
        patches = []
        session = fixtures["demoSession"]
        node = fixtures["demoNodes"][0]
        async def handle(route):
            req = route.request
            path = req.url.split("/ca/ht", 1)[1].split("?", 1)[0]
            status, data, error = 200, None, None
            if path == "/hh": data = session
            elif path == "/jd/" + node["id"] and req.method == "PATCH":
                patches.append({"body": req.post_data_json, "headers": req.headers})
                if len(patches) == 1:
                    status, error = 401, {"code": "ADMIN_SESSION_INVALID", "message": "登录已过期"}
                else: data = {**node, **req.post_data_json, "config_version": 2}
            elif path == "/jd/" + node["id"]: data = node
            elif path == "/jd": data = fixtures["demoNodes"]
            elif path == "/fwq": data = fixtures["demoMachines"]
            elif path == "/ly": data = fixtures["demoRoutes"]
            elif path.startswith("/sz/"): data = []
            else: data = []
            await route.fulfill(status=status, content_type="application/json", body=json.dumps({"data": data, "error": error, "meta": {"request_id": "browser-isolated", "config_version": 1}}))
        await page.route("**/ca/ht/**", handle)
        await page.goto(BASE + "/nodes/" + node["id"] + "/edit")
        await page.locator("#name").wait_for()
        await page.locator("#name").fill("会话恢复保留的输入")
        await page.get_by_role("button", name="保存草稿", exact=True).click()
        await page.get_by_role("heading", name="管理员登录").wait_for()
        await page.screenshot(path=str(OUT / "reauthentication-1440-light.png"))
        await page.get_by_label("邮箱", exact=False).fill("admin@cpanel.local")
        await page.locator(".login-form input[type=password]").fill("isolated-test-password")
        await page.get_by_role("button", name="登录后台", exact=True).click()
        await page.locator("#name").wait_for(state="visible")
        assert await page.locator("#name").input_value() == "会话恢复保留的输入"
        assert len(patches) == 1
        assert not await page.evaluate("JSON.stringify({...localStorage,...sessionStorage}).includes('会话恢复保留的输入')")
        await page.get_by_role("button", name="保存草稿", exact=True).click()
        await page.wait_for_url("**/nodes")
        assert len(patches) == 2 and patches[1]["headers"]["if-match"] == f'"{node["config_version"]}"'
        report["behaviors"]["same_admin_reauthentication_preserves_memory_only_input_and_requires_manual_retry"] = True
        await live.close()
        await browser.close()
    (OUT / "browser-results.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    assert not report["errors"], report["errors"]
    assert not any(item["overflow"] for item in report["screens"]), "horizontal overflow"
    print(json.dumps({"screens": len(report["screens"]), "errors": report["errors"], "behaviors": report["behaviors"]}, ensure_ascii=False))

if __name__ == "__main__":
    asyncio.run(main())
