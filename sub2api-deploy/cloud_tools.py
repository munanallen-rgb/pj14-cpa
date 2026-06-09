#!/usr/bin/env python3
import argparse
import json
import os
import secrets
import sys
import urllib.error
import urllib.request
from pathlib import Path


CPA_CONFIG_PATHS = {
    "cpa1": "config.yaml",
    "cpa2": "instances/cpa2/config.yaml",
    "cpa3": "instances/cpa3/config.yaml",
}


def read_cpa_api_key(path):
    config_path = Path(path)
    if not config_path.exists():
        raise SystemExit(f"CPA config file is missing: {config_path}")

    for line in config_path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped.startswith("-"):
            value = stripped[1:].strip().strip('"').strip("'")
            if value:
                return value
    raise SystemExit(f"No api-keys entry found in {config_path}")


def resolve_cpa_keys(args):
    return {
        "cpa1": args.cpa1_key or read_cpa_api_key(args.cpa1_config),
        "cpa2": args.cpa2_key or read_cpa_api_key(args.cpa2_config),
        "cpa3": args.cpa3_key or read_cpa_api_key(args.cpa3_config),
    }


def read_dotenv(path):
    values = {}
    if not path or not Path(path).exists():
        return values
    for line in Path(path).read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or "=" not in stripped:
            continue
        key, value = stripped.split("=", 1)
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
            value = value[1:-1]
        values[key.strip()] = value
    return values


def request_json(base_url, method, path, token=None, body=None, timeout=90):
    url = base_url.rstrip("/") + path
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = f"Bearer {token}"

    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            raw = response.read().decode("utf-8")
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as err:
        raw = err.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{method} {url} failed with HTTP {err.code}: {raw}") from err
    except urllib.error.URLError as err:
        raise RuntimeError(f"{method} {url} failed: {err}") from err


def data_items(response):
    data = response.get("data")
    if data is None:
        return []
    if isinstance(data, dict) and "items" in data:
        return data["items"] or []
    return [data]


def login(base_url, email, password):
    response = request_json(
        base_url,
        "POST",
        "/api/v1/auth/login",
        body={"email": email, "password": password},
    )
    return response["data"]["access_token"]


def cmd_generate_env(args):
    output = Path(args.output)
    if output.exists() and not args.force:
        raise SystemExit(f"Refusing to overwrite existing file: {output}. Use --force.")
    output.parent.mkdir(parents=True, exist_ok=True)

    values = {
        "SUB2API_BIND_HOST": args.bind_host,
        "SUB2API_PORT": str(args.port),
        "SUB2API_IMAGE": "weishaw/sub2api:latest",
        "ADMIN_EMAIL": args.admin_email,
        "ADMIN_PASSWORD": secrets.token_hex(32),
        "POSTGRES_USER": "sub2api",
        "POSTGRES_PASSWORD": secrets.token_hex(32),
        "POSTGRES_DB": "sub2api",
        "JWT_SECRET": secrets.token_hex(32),
        "TOTP_ENCRYPTION_KEY": secrets.token_hex(32),
        "TZ": "Asia/Shanghai",
        "CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY": "replace-with-cpa-management-key",
        "CPA_QUOTA_COLLECTOR_INSTANCES": "cpa1=http://cpa1:8317,cpa2=http://cpa2:8317,cpa3=http://cpa3:8317",
        "CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA1": "",
        "CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA2": "",
        "CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA3": "",
        "PORTAL_BIND_HOST": "0.0.0.0",
        "PORTAL_PORT": "18100",
        "PORTAL_PUBLIC_SUB2API_BASE_URL": f"http://<server-ip>:{args.port}",
        "PORTAL_SESSION_SECRET": secrets.token_hex(32),
        "PORTAL_SESSION_TTL_HOURS": "24",
        "PORTAL_COOKIE_SECURE": "false",
        "PORTAL_ALLOWED_ORIGINS": "",
        "PORTAL_BOOTSTRAP_ADMIN_EMAIL": "portal-admin@sub2api.local",
        "PORTAL_BOOTSTRAP_ADMIN_PASSWORD": secrets.token_hex(32),
        "PORTAL_SUB2API_ADMIN_EMAIL": "portal-service@sub2api.local",
        "PORTAL_SUB2API_ADMIN_PASSWORD": secrets.token_hex(32),
        "PORTAL_SUB2API_DEFAULT_GROUP_NAME": "cpa-openai",
        "PORTAL_SUB2API_DEFAULT_KEY_QUOTA": "0",
        "SECURITY_URL_ALLOWLIST_ENABLED": "false",
        "SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP": "true",
        "SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS": "true",
    }

    content = "\n".join(f"{key}={value}" for key, value in values.items()) + "\n"
    output.write_text(content, encoding="utf-8")

    print(json.dumps({
        "output": str(output),
        "admin_email": values["ADMIN_EMAIL"],
        "admin_password": values["ADMIN_PASSWORD"],
        "portal_url": f"http://<server-ip>:{values['PORTAL_PORT']}",
        "portal_admin_email": values["PORTAL_BOOTSTRAP_ADMIN_EMAIL"],
        "portal_admin_password": values["PORTAL_BOOTSTRAP_ADMIN_PASSWORD"],
        "portal_sub2api_admin_email": values["PORTAL_SUB2API_ADMIN_EMAIL"],
        "portal_sub2api_admin_password": values["PORTAL_SUB2API_ADMIN_PASSWORD"],
    }, indent=2))


def upstream_urls(mode):
    if mode == "cloud":
        return {
            "cpa1": "http://cpa1:8317/v1",
            "cpa2": "http://cpa2:8317/v1",
            "cpa3": "http://cpa3:8317/v1",
        }
    return {
        "cpa1": "http://host.docker.internal:8317/v1",
        "cpa2": "http://host.docker.internal:8318/v1",
        "cpa3": "http://host.docker.internal:8319/v1",
    }


def cmd_bootstrap(args):
    env_values = read_dotenv(args.env_file)
    email = args.admin_email or os.environ.get("ADMIN_EMAIL") or env_values.get("ADMIN_EMAIL") or "admin@sub2api.local"
    password = args.admin_password or os.environ.get("ADMIN_PASSWORD") or env_values.get("ADMIN_PASSWORD") or ""
    if not password:
        raise SystemExit("ADMIN_PASSWORD is required via --admin-password, environment, or --env-file.")

    token = login(args.base_url, email, password)
    urls = upstream_urls(args.upstream_mode)
    cpa_keys = resolve_cpa_keys(args)

    groups = data_items(request_json(args.base_url, "GET", "/api/v1/admin/groups", token=token))
    group = next((item for item in groups if item.get("name") == args.group_name), None)
    if group is None:
        group = request_json(args.base_url, "POST", "/api/v1/admin/groups", token=token, body={
            "name": args.group_name,
            "description": "CPA OpenAI-compatible upstream group",
            "platform": "openai",
            "rate_multiplier": 1,
            "is_exclusive": False,
            "status": "active",
            "subscription_type": "standard",
            "allow_image_generation": True,
        })["data"]
        group_action = "created"
    else:
        group_action = "existing"
    group_id = int(group["id"])

    accounts = data_items(request_json(args.base_url, "GET", "/api/v1/admin/accounts", token=token))
    account_results = []
    for name in ("cpa1", "cpa2", "cpa3"):
        payload = {
            "name": name,
            "notes": f"{name.upper()} {args.upstream_mode} upstream",
            "platform": "openai",
            "type": "apikey",
            "credentials": {"base_url": urls[name], "api_key": cpa_keys[name]},
            "extra": {"openai_responses_supported": True},
            "concurrency": 10,
            "priority": 1,
            "rate_multiplier": 1,
            "status": "active",
            "group_ids": [group_id],
        }
        existing = next((item for item in accounts if item.get("name") == name), None)
        if existing:
            account = request_json(args.base_url, "PUT", f"/api/v1/admin/accounts/{existing['id']}", token=token, body=payload)["data"]
            action = "updated"
        else:
            account = request_json(args.base_url, "POST", "/api/v1/admin/accounts", token=token, body=payload)["data"]
            action = "created"
        account_results.append({"name": name, "id": account.get("id"), "action": action, "base_url": urls[name]})

    channels = data_items(request_json(args.base_url, "GET", "/api/v1/admin/channels", token=token))
    channel_payload = {
        "name": args.channel_name,
        "description": "Routes OpenAI-compatible requests to the CPA pool",
        "status": "active",
        "billing_model_source": "channel_mapped",
        "restrict_models": False,
        "features": "",
        "features_config": {"codex_image_generation_bridge": {"openai": True}},
        "group_ids": [group_id],
        "model_pricing": [],
        "model_mapping": {},
        "apply_pricing_to_account_stats": False,
        "account_stats_pricing_rules": [],
    }
    channel = next((item for item in channels if item.get("name") == args.channel_name), None)
    if channel:
        channel = request_json(args.base_url, "PUT", f"/api/v1/admin/channels/{channel['id']}", token=token, body=channel_payload)["data"]
        channel_action = "updated"
    else:
        channel = request_json(args.base_url, "POST", "/api/v1/admin/channels", token=token, body=channel_payload)["data"]
        channel_action = "created"

    user = request_json(args.base_url, "GET", "/api/v1/admin/users/1", token=token)["data"]
    if float(user.get("balance") or 0) < args.user_balance:
        user = request_json(args.base_url, "POST", "/api/v1/admin/users/1/balance", token=token, body={
            "balance": args.user_balance,
            "operation": "set",
            "notes": "CPA pool bootstrap",
        })["data"]
        balance_action = "set"
    else:
        balance_action = "existing"

    keys = data_items(request_json(args.base_url, "GET", "/api/v1/keys", token=token))
    key = next((item for item in keys if item.get("name") == args.key_name and int(item.get("group_id") or 0) == group_id), None)
    if key:
        key_action = "existing"
    else:
        key = request_json(args.base_url, "POST", "/api/v1/keys", token=token, body={
            "name": args.key_name,
            "group_id": group_id,
            "quota": args.key_quota,
            "status": "active",
        })["data"]
        key_action = "created"

    print(json.dumps({
        "base_url": f"{args.base_url.rstrip('/')}/v1",
        "group": {"id": group_id, "name": args.group_name, "action": group_action},
        "accounts": account_results,
        "channel": {"id": channel.get("id"), "name": args.channel_name, "action": channel_action},
        "balance": {"value": user.get("balance"), "action": balance_action},
        "key": {"name": args.key_name, "key": key.get("key"), "action": key_action},
    }, indent=2))


def check_models(name, base_url, api_key):
    try:
        response = request_json(base_url, "GET", "/models", token=api_key, timeout=30)
        models = response.get("data") or []
        return {"instance": name, "ok": True, "model_count": len(models), "first_model": models[0].get("id") if models else "", "error": ""}
    except Exception as exc:
        return {"instance": name, "ok": False, "model_count": 0, "first_model": "", "error": str(exc)}


def cmd_verify(args):
    cpa = []
    if not args.skip_direct_cpa_checks:
        cpa_keys = resolve_cpa_keys(args)
        cpa = [
            check_models("cpa1", args.cpa1_base_url, cpa_keys["cpa1"]),
            check_models("cpa2", args.cpa2_base_url, cpa_keys["cpa2"]),
            check_models("cpa3", args.cpa3_base_url, cpa_keys["cpa3"]),
        ]

    try:
        health_response = request_json(args.sub2api_url, "GET", "/health", timeout=30)
        health = {"ok": True, "status": health_response.get("status"), "error": ""}
    except Exception as exc:
        health = {"ok": False, "status": "", "error": str(exc)}

    sub2api_key = args.sub2api_key or os.environ.get("SUB2API_API_KEY") or ""
    if sub2api_key:
        try:
            chat_response = request_json(args.sub2api_url, "POST", "/v1/chat/completions", token=sub2api_key, body={
                "model": args.model,
                "messages": [{"role": "user", "content": "Say hello in one short sentence."}],
            })
            choice = (chat_response.get("choices") or [{}])[0]
            message = choice.get("message") or {}
            chat = {
                "ok": True,
                "status": 200,
                "model": chat_response.get("model"),
                "content": message.get("content", ""),
                "id": chat_response.get("id"),
                "skipped": False,
                "error": "",
            }
        except Exception as exc:
            chat = {"ok": False, "status": 0, "model": args.model, "content": "", "id": "", "skipped": False, "error": str(exc)}
    else:
        chat = {
            "ok": True,
            "status": 0,
            "model": args.model,
            "content": "",
            "id": "",
            "skipped": True,
            "error": "Skipped because no Sub2API key was provided. Set --sub2api-key or SUB2API_API_KEY for chat verification.",
        }

    all_cpa_ready = True
    if not args.skip_direct_cpa_checks:
        all_cpa_ready = all(item["ok"] and item["model_count"] > 0 for item in cpa)

    summary = {
        "direct_cpa_checks_skipped": args.skip_direct_cpa_checks,
        "all_cpa_ready": all_cpa_ready,
        "sub2api_ready": health["ok"] and chat["ok"],
        "cpa": cpa,
        "sub2api_health": health,
        "sub2api_chat": chat,
    }
    print(json.dumps(summary, indent=2))

    if not summary["sub2api_ready"]:
        return 1
    if args.require_all_cpa_models and not all_cpa_ready:
        return 2
    return 0


def build_parser():
    parser = argparse.ArgumentParser(description="CPA + Sub2API cloud helper")
    sub = parser.add_subparsers(dest="command", required=True)

    gen = sub.add_parser("generate-env")
    gen.add_argument("--output", default="temp/cpa-sub2api-cloud.env")
    gen.add_argument("--admin-email", default="admin@sub2api.local")
    gen.add_argument("--bind-host", default="0.0.0.0")
    gen.add_argument("--port", type=int, default=18080)
    gen.add_argument("--force", action="store_true")
    gen.set_defaults(func=cmd_generate_env)

    boot = sub.add_parser("bootstrap")
    boot.add_argument("--base-url", default="http://127.0.0.1:18080")
    boot.add_argument("--env-file", default=".env")
    boot.add_argument("--admin-email", default="")
    boot.add_argument("--admin-password", default="")
    boot.add_argument("--upstream-mode", choices=("local", "cloud"), default="cloud")
    boot.add_argument("--group-name", default="cpa-openai")
    boot.add_argument("--channel-name", default="cpa-openai-channel")
    boot.add_argument("--key-name", default="local-cpa-pool")
    boot.add_argument("--user-balance", type=float, default=1000)
    boot.add_argument("--key-quota", type=float, default=100)
    boot.add_argument("--cpa1-config", default=CPA_CONFIG_PATHS["cpa1"])
    boot.add_argument("--cpa2-config", default=CPA_CONFIG_PATHS["cpa2"])
    boot.add_argument("--cpa3-config", default=CPA_CONFIG_PATHS["cpa3"])
    boot.add_argument("--cpa1-key", default="")
    boot.add_argument("--cpa2-key", default="")
    boot.add_argument("--cpa3-key", default="")
    boot.set_defaults(func=cmd_bootstrap)

    verify = sub.add_parser("verify")
    verify.add_argument("--sub2api-url", default="http://127.0.0.1:18080")
    verify.add_argument("--sub2api-key", default="")
    verify.add_argument("--cpa1-base-url", default="http://127.0.0.1:8317/v1")
    verify.add_argument("--cpa2-base-url", default="http://127.0.0.1:8318/v1")
    verify.add_argument("--cpa3-base-url", default="http://127.0.0.1:8319/v1")
    verify.add_argument("--cpa1-config", default=CPA_CONFIG_PATHS["cpa1"])
    verify.add_argument("--cpa2-config", default=CPA_CONFIG_PATHS["cpa2"])
    verify.add_argument("--cpa3-config", default=CPA_CONFIG_PATHS["cpa3"])
    verify.add_argument("--cpa1-key", default="")
    verify.add_argument("--cpa2-key", default="")
    verify.add_argument("--cpa3-key", default="")
    verify.add_argument("--model", default="gpt-5.4-mini")
    verify.add_argument("--require-all-cpa-models", action="store_true")
    verify.add_argument("--skip-direct-cpa-checks", action="store_true")
    verify.set_defaults(func=cmd_verify)

    return parser


def main():
    parser = build_parser()
    args = parser.parse_args()
    result = args.func(args)
    return result if isinstance(result, int) else 0


if __name__ == "__main__":
    sys.exit(main())
