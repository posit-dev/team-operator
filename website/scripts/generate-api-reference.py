"""
Generate docs/api-reference.md from Team Operator CRD schemas.

USAGE:
    python website/scripts/generate-api-reference.py [--crd-dir DIR] [--output FILE]

Defaults resolve relative to the repo root so the script works from either
the repo root or the website/ directory. Requires: pyyaml
"""

import argparse, glob, os, sys

try:
    import yaml
except ImportError:
    sys.exit("PyYAML is required. Install with: pip install pyyaml")

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

CRD_ORDER = ["sites", "connects", "workbenches", "packagemanagers", "chronicles", "postgresdatabases", "flightdecks"]


def resolve_path(p):
    if os.path.isabs(p):
        return p
    for base in (os.getcwd(), REPO_ROOT):
        c = os.path.join(base, p)
        if os.path.exists(c):
            return c
    return os.path.join(REPO_ROOT, p)


def type_label(prop):
    t = prop.get("type", "")
    fmt = prop.get("format", "")
    additional = prop.get("additionalProperties")
    items = prop.get("items", {})
    if t == "array":
        return f"`[]{items.get('type', 'object') if items else 'object'}`"
    if t == "object" and additional:
        vt = additional.get("type", "string") if isinstance(additional, dict) else "string"
        return f"`map[string]{vt}`"
    if t == "integer":
        return f"`int{'64' if fmt == 'int64' else ''}`"
    if t in ("object", "boolean"):
        return f"`{'bool' if t == 'boolean' else t}`"
    if t:
        return f"`{t}`"
    return "`object`" if "properties" in prop else "`any`"


def clean(s):
    return " ".join(s.split()) if s else ""


def extract_fields(props, required, prefix, depth, max_depth):
    rows = []
    for name, prop in sorted(props.items()):
        path = f"{prefix}.{name}"
        rows.append({
            "field": f"`{path}`",
            "type": type_label(prop),
            "required": "**Yes**" if name in (required or []) else "No",
            "description": clean(prop.get("description", "")),
        })
        if depth < max_depth and prop.get("type") == "object" and "properties" in prop:
            rows.extend(extract_fields(prop["properties"], prop.get("required", []), path, depth + 1, max_depth))
    return rows


def render_table(rows):
    lines = ["| Field | Type | Required | Description |", "|-------|------|----------|-------------|"]
    for r in rows:
        lines.append(f"| {r['field']} | {r['type']} | {r['required']} | {r['description'].replace('|', chr(92) + '|')} |")
    return "\n".join(lines)


def crd_to_section(crd):
    names = crd["spec"]["names"]
    kind = names["kind"]
    plural = names.get("plural", kind.lower() + "s")
    short_names = names.get("shortNames", [])
    group = crd["spec"]["group"]
    scope = crd["spec"].get("scope", "Namespaced")

    ver = crd["spec"]["versions"][0]
    version_name = ver["name"]
    openapi = ver.get("schema", {}).get("openAPIV3Schema", {}).get("properties", {})
    spec_props_def = openapi.get("spec", {})
    status_props_def = openapi.get("status", {})

    lines = [f"## {kind}", "",
             f"**API Group/Version:** `{group}/{version_name}`",
             f"**Kind:** `{kind}`",
             f"**Plural:** `{plural}`"]
    if short_names:
        lines.append(f"**Short Names:** {', '.join(f'`{s}`' for s in short_names)}")
    lines += [f"**Scope:** {scope}", ""]

    if spec_props := spec_props_def.get("properties"):
        lines += ["### Spec Fields", "", render_table(
            extract_fields(spec_props, spec_props_def.get("required", []), ".spec", 0, 1)
        ), ""]

    if status_props := status_props_def.get("properties"):
        lines += ["### Status Fields", "", render_table(
            extract_fields(status_props, [], ".status", 0, 0)
        ), ""]

    lines += ["---", ""]
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--crd-dir", default="config/crd/bases")
    parser.add_argument("--output", default="docs/api-reference.md")
    args = parser.parse_args()

    crd_dir = resolve_path(args.crd_dir)
    output_path = resolve_path(args.output)

    if not os.path.isdir(crd_dir):
        sys.exit(f"CRD directory not found: {crd_dir}")

    crd_files = glob.glob(os.path.join(crd_dir, "core.posit.team_*.yaml"))
    if not crd_files:
        sys.exit(f"No CRD files found in: {crd_dir}")

    crds = []
    for path in crd_files:
        with open(path) as f:
            crd = yaml.safe_load(f)
        crds.append((crd["spec"]["names"].get("plural", ""), crd))

    crds.sort(key=lambda x: CRD_ORDER.index(x[0]) if x[0] in CRD_ORDER else len(CRD_ORDER))

    toc = [f"- [{crd['spec']['names']['kind']}](#{crd['spec']['names']['kind'].lower()})" for _, crd in crds]

    header = [
        "---",
        "title: API Reference",
        "description: Complete CRD field reference for Team Operator resources (auto-generated from CRD schemas)",
        "---",
        "",
        "# Team Operator API Reference",
        "",
        "Auto-generated from CRD schemas.",
        "",
        "**API Group:** `core.posit.team`",
        "",
        "## Table of Contents",
        "",
        *toc,
        "",
        "---",
        "",
    ]

    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        f.write("\n".join(header))
        for _, crd in crds:
            f.write(crd_to_section(crd))

    print(f"Generated {output_path} from {len(crds)} CRDs in {crd_dir}")


if __name__ == "__main__":
    main()
