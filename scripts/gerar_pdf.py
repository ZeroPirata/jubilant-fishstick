#!/usr/bin/env python3
"""
Recebe JSON via stdin com campos:
  - curriculo: texto do currículo
  - cover_letter: texto da carta de apresentação
  - output_dir: diretório onde os PDFs serão salvos

Retorna JSON via stdout com:
  - resume_path: caminho do PDF do currículo
  - cover_letter_path: caminho do PDF da carta
  - error: mensagem de erro (se houver)
"""

import json
import os
import re
import sys
import traceback

from weasyprint import HTML, CSS


# ── Section name sets ──────────────────────────────────────────────────────────
_ALL_SECTIONS = frozenset({
    "SUMMARY", "EXPERIENCE", "SKILLS", "EDUCATION", "PROJECTS",
    "RESUMO PROFISSIONAL",
    "EXPERIÊNCIA PROFISSIONAL", "EXPERIENCIA PROFISSIONAL",
    "HABILIDADES TÉCNICAS", "HABILIDADES TECNICAS",
    "FORMAÇÃO ACADÊMICA", "FORMACAO ACADEMICA",
    "PROJETOS",
    "CERTIFICAÇÕES", "CERTIFICACOES", "CERTIFICATIONS",
})
_EXP_SECTIONS   = frozenset({"EXPERIENCE", "EXPERIÊNCIA PROFISSIONAL", "EXPERIENCIA PROFISSIONAL"})
_PROJ_SECTIONS  = frozenset({"PROJECTS", "PROJETOS"})
_EDU_SECTIONS   = frozenset({"EDUCATION", "FORMAÇÃO ACADÊMICA", "FORMACAO ACADEMICA"})
_SKILL_SECTIONS = frozenset({"SKILLS", "HABILIDADES TÉCNICAS", "HABILIDADES TECNICAS"})
_CERT_SECTIONS  = frozenset({"CERTIFICATIONS", "CERTIFICAÇÕES", "CERTIFICACOES"})

# ── CSS ────────────────────────────────────────────────────────────────────────
RESUME_CSS = CSS(string="""
@page { margin: 1.5cm 1.8cm; size: A4; }

* { box-sizing: border-box; margin: 0; padding: 0; }

body {
    font-family: 'Liberation Sans', 'Arial', 'Helvetica Neue', sans-serif;
    font-size: 10pt;
    line-height: 1.4;
    color: #1f1f1f;
}

a { color: #666; text-decoration: none; }

/* ── Header ── */
.header {
    border-bottom: 2px solid #1a3a5c;
    padding-bottom: 6pt;
    margin-bottom: 8pt;
}

.name {
    font-size: 20pt;
    font-weight: bold;
    color: #1a3a5c;
    letter-spacing: 0.2pt;
    margin-bottom: 4pt;
}

.contact      { font-size: 8.5pt; color: #555; margin-bottom: 1pt; }
.contact-web  { font-size: 8.5pt; color: #555; }
.sep          { color: #ccc; margin: 0 3pt; }

/* ── Section headers ── */
h2 {
    font-size: 8pt;
    font-weight: bold;
    text-transform: uppercase;
    letter-spacing: 1.5pt;
    color: #1a3a5c;
    margin-top: 9pt;
    margin-bottom: 4pt;
    border-bottom: 1px solid #1a3a5c;
    padding-bottom: 2pt;
}

/* ── Experience ── */
.exp-role {
    font-weight: bold;
    font-size: 10.5pt;
    color: #1a3a5c;
    margin-top: 6pt;
    margin-bottom: 1pt;
}

.exp-meta { font-size: 8.5pt; color: #1a3a5c; margin-bottom: 3pt; }

/* ── Bullets ── */
ul { padding-left: 11pt; margin: 2pt 0 3pt 0; }
li { font-size: 9.5pt; margin-bottom: 1pt; line-height: 1.4; text-align: justify; }

/* ── Skills ── */
.skill-line { font-size: 9.5pt; margin: 1.5pt 0; }
.skill-key  { font-weight: bold; color: #1a3a5c; }

/* ── Projects ── */
.proj-name {
    font-weight: bold;
    font-size: 10.5pt;
    color: #1a3a5c;
    margin-top: 7pt;
    margin-bottom: 1pt;
}

.proj-meta  { font-size: 8.5pt; color: #1a3a5c; margin-bottom: 2pt; }
.proj-desc  { font-size: 9.5pt; line-height: 1.4; color: #333; margin-top: 1pt; text-align: justify; }

/* ── Education ── */
.edu-course {
    font-weight: bold;
    font-size: 10.5pt;
    color: #1a3a5c;
    margin-top: 6pt;
    margin-bottom: 1pt;
}

.edu-institution { font-size: 8.5pt; color: #1a3a5c; margin-bottom: 1pt; }

/* ── Certifications (fonte reduzida para comportar muitos itens) ── */
.cert-name {
    font-weight: bold;
    font-size: 9pt;
    color: #1a3a5c;
    margin-top: 4pt;
    margin-bottom: 0.5pt;
}

.cert-name a, a.cert-name-link {
    color: #1a3a5c;
    text-decoration: none;
    font-weight: bold;
    font-size: 9pt;
}

.cert-meta { font-size: 7.5pt; color: #1a3a5c; margin-bottom: 2pt; }

/* ── Generic ── */
p { font-size: 9.5pt; margin: 2pt 0; line-height: 1.45; text-align: justify; }
""")


COVER_CSS = CSS(string="""
@page { margin: 3cm 2.5cm; size: A4; }

* { box-sizing: border-box; margin: 0; padding: 0; }

body {
    font-family: 'Liberation Sans', 'Arial', 'Helvetica Neue', sans-serif;
    font-size: 11pt;
    line-height: 1.65;
    color: #1f1f1f;
}

a           { color: #555; text-decoration: none; }
p           { margin-bottom: 16pt; }
.salutation { margin-bottom: 22pt; color: #444; }
""")


# ── HTML helpers ───────────────────────────────────────────────────────────────

def _esc(s: str) -> str:
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def _url_label(url: str) -> str:
    """Return a short friendly label for a contact URL."""
    u = url.lower()
    if 'linkedin.com' in u:
        return 'LinkedIn'
    if 'github.com' in u or 'github.io' in u:
        return 'GitHub'
    if 'twitter.com' in u or 'x.com' in u:
        return 'Twitter'
    if 'gitlab.com' in u:
        return 'GitLab'
    # Known portfolio/personal hosting platforms → label as Portfólio
    if any(h in u for h in ('vercel.app', 'netlify.app', 'github.io', 'heroku.com', 'render.com')):
        return 'Portfólio'
    # Generic unknown URL → treat as portfolio in resume context
    return 'Portfólio'


def _linkify_and_esc(s: str) -> str:
    """Escape HTML but wrap bare URLs in <a> tags (full URL as label)."""
    parts = re.split(r'(https?://\S+)', s)
    result = []
    for part in parts:
        if part.startswith("http://") or part.startswith("https://"):
            url = _esc(part.rstrip(".,)>"))
            result.append(f'<a href="{url}">{url}</a>')
        else:
            result.append(_esc(part))
    return "".join(result)


def _contact_linkify(s: str) -> str:
    """Like _linkify_and_esc but uses friendly label instead of raw URL for contact lines."""
    parts = re.split(r'(https?://\S+)', s)
    result = []
    for part in parts:
        if part.startswith("http://") or part.startswith("https://"):
            url_clean = part.rstrip(".,)>")
            url_esc = _esc(url_clean)
            label = _esc(_url_label(url_clean))
            result.append(f'<a href="{url_esc}">{label}</a>')
        else:
            result.append(_esc(part))
    return "".join(result)


def _inline(s: str) -> str:
    s = _linkify_and_esc(s)
    s = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", s)
    s = re.sub(r"\*(.+?)\*", r"<em>\1</em>", s)
    s = re.sub(r"`(.+?)`", r"<code>\1</code>", s)
    return s


def _is_section(line: str) -> bool:
    return line.strip().upper() in _ALL_SECTIONS


def _is_skill_line(line: str) -> bool:
    s = line.strip()
    if not s or s.startswith("-") or ":" not in s:
        return False
    idx = s.index(":")
    key = s[:idx].strip()
    val = s[idx + 1:].strip()
    _CONTACT_LABELS = {"email", "e-mail", "phone", "telefone", "nome", "name",
                       "linkedin", "github", "portfolio", "site", "website"}
    if key.lower() in _CONTACT_LABELS:
        return False
    return bool(key) and bool(val) and len(key.split()) <= 4


def _contact_group(part: str) -> int:
    """0 = basic (email/phone), 1 = web (github/linkedin/portfolio)."""
    p = part.lower()
    if "@" in p:
        return 0
    if re.match(r'^[\d\s\+\-\(\)\.]+$', part.strip()) and len(part.strip()) < 20:
        return 0
    return 1


# ── Project helpers ────────────────────────────────────────────────────────────

def _render_proj_buffer(html: list, lines: list[str]) -> None:
    """Interpret a block of raw project lines and render the project HTML.

    Handles any ordering the LLM may produce:
      - Line 0: project name (optionally "Name — Company" with em-dash)
      - Bare URL lines           → link
      - "desc | url" lines       → description + link
      - Short lines w/o periods  → company (if company not yet set)
      - Everything else          → description (first one wins)
    """
    lines = [l.strip() for l in lines if l.strip()]
    if not lines:
        return

    # Parse name (and optional inline company / description via em-dashes)
    # LLM may produce: "Name — Company — Description" all on the first line.
    first = lines[0]
    desc = ''
    link = None
    if '\u2014' in first:
        name, _, rest = first.partition('\u2014')
        name = name.strip()
        if '\u2014' in rest:
            # "Name — Company — Description"
            company_part, _, desc_part = rest.partition('\u2014')
            company = company_part.strip() or None
            desc    = desc_part.strip()
        else:
            company = rest.strip() or None
    else:
        name    = first
        company = None

    _LINK_LABELS = {'github', 'gitlab', 'linkedin', 'twitter', 'portfolio',
                    'portfólio', 'portifolio', 'demo', 'site', 'link', 'repositório', 'repo'}

    desc_parts: list[str] = []

    for line in lines[1:]:
        # Bare URL → link (signals end, but keep looping to collect any stray lines)
        if line.startswith('http://') or line.startswith('https://'):
            if not link:
                link = line.rstrip('.,)')
            continue
        # Inline link: "description fragment | https://..."
        m = re.search(r'\s*\|\s*(https?://\S+)\s*$', line)
        if m:
            if not link:
                link = m.group(1).rstrip('.,)')
            fragment = line[:m.start()].strip()
            if fragment:
                desc_parts.append(fragment)
            continue
        # Standalone label word without URL (e.g. "GitHub", "GitLab") → skip silently
        if line.strip().lower() in _LINK_LABELS:
            continue
        # Trailing "— GitHub" / "— GitLab" etc. without a real URL → strip the label
        if '\u2014' in line:
            left_part, _, right_part = line.partition('\u2014')
            if right_part.strip().lower() in _LINK_LABELS:
                line = left_part.strip()
                if not line:
                    continue
        # Company: short line, no sentence punctuation, not yet set, no desc parts yet
        if (not company and not desc_parts and len(line) < 70
                and not any(c in line for c in '.!?')
                and not line.startswith('-')):
            company = line
            continue
        # Description fragment — collect all, join later
        desc_parts.append(line)

    # Join multi-line description fragments with a space
    if desc_parts:
        desc = ' '.join(desc_parts)

    html.append(f'<div class="proj-name">{_esc(name)}</div>')

    meta_parts: list[str] = []
    if company:
        meta_parts.append(_esc(company))
    if link:
        link_esc = _esc(link)
        label    = _esc(_url_label(link))
        meta_parts.append(f'<a href="{link_esc}">{label}</a>')
    if meta_parts:
        html.append(f'<div class="proj-meta">{" \u2014 ".join(meta_parts)}</div>')

    if desc:
        html.append(f'<div class="proj-desc">{_inline(desc)}</div>')


# ── Main resume parser ─────────────────────────────────────────────────────────

def curriculo_para_html(texto: str) -> str:
    lines = texto.split("\n")
    html: list = []
    i, n = 0, len(lines)

    # Skip leading blanks
    while i < n and not lines[i].strip():
        i += 1

    # ── HEADER: name ──
    html.append('<div class="header">')
    if i < n:
        name = re.sub(r'^nome\s*:\s*', '', lines[i].strip(), flags=re.IGNORECASE)
        html.append(f'<div class="name">{_esc(name)}</div>')
        i += 1

    # ── HEADER: contact lines ──
    # Collect all parts from lines with | / @ / url patterns until blank/section
    raw_parts: list[str] = []
    while i < n:
        line = lines[i].strip()
        if not line or _is_section(line):
            break
        is_contact = ("|" in line or "@" in line
                      or re.search(r'(linkedin|github|http|portf)', line, re.IGNORECASE))
        if is_contact:
            for part in re.split(r'\s*\|\s*', line):
                part = re.sub(
                    r'^(email|e-mail|linkedin|github|portf[oó]lio|portfolio|site|phone|telefone)\s*:\s*',
                    '', part.strip(), flags=re.IGNORECASE
                ).strip()
                if part:
                    raw_parts.append(part)
            i += 1
        else:
            break

    # Split into two groups: basic (email/phone) and web (social/portfolio)
    group0 = [p for p in raw_parts if _contact_group(p) == 0]
    group1 = [p for p in raw_parts if _contact_group(p) == 1]

    sep = ' <span class="sep">|</span> '
    if group0:
        html.append(f'<div class="contact">{sep.join(_contact_linkify(p) for p in group0)}</div>')
    if group1:
        html.append(f'<div class="contact-web">{sep.join(_contact_linkify(p) for p in group1)}</div>')

    html.append('</div>')  # .header

    # ── BODY ──
    current_section: str | None = None
    in_ul        = False
    expect_date  = False

    # Experience pending state
    exp_company: str | None = None
    exp_role:    str | None = None

    # Education pending state
    edu_institution: str | None = None
    edu_course:      str | None = None

    # Project buffer — accumulates all lines of a project block; flushed on blank / section change
    proj_buffer: list[str] = []

    # Certification pending state — buffered until meta line (issuer — date)
    pending_cert: dict | None = None  # {'name': str, 'link': str|None}

    # Skill bullet-list state — LLM may emit "Category:\n• item1\n• item2" instead of one-liner
    skill_key:   str | None = None
    skill_items: list[str]  = []

    def flush_skill() -> None:
        nonlocal skill_key, skill_items
        if skill_key is None:
            return
        val = ', '.join(skill_items)
        if val:
            html.append(
                f'<p class="skill-line">'
                f'<span class="skill-key">{_esc(skill_key)}:</span> {_esc(val)}'
                f'</p>'
            )
        skill_key = None
        skill_items = []

    def close_ul() -> None:
        nonlocal in_ul
        if in_ul:
            html.append('</ul>')
            in_ul = False

    def flush_cert() -> None:
        nonlocal pending_cert
        if pending_cert is None:
            return
        name = pending_cert['name']
        link = pending_cert.get('link')
        if link:
            link_url = _esc(link.rstrip(".,)>"))
            name_rendered = f'<a class="cert-name-link" href="{link_url}">{_esc(name)}</a>'
        else:
            name_rendered = _esc(name)
        html.append(f'<div class="cert-name">{name_rendered}</div>')
        pending_cert = None

    def flush_exp(date: str = '') -> None:
        nonlocal exp_company, exp_role
        if exp_role is None:
            return
        html.append(f'<div class="exp-role">{_esc(exp_role)}</div>')
        meta_parts = []
        if exp_company:
            meta_parts.append(_esc(exp_company))
        if date:
            meta_parts.append(_esc(date))
        if meta_parts:
            html.append(f'<div class="exp-meta">{" \u2014 ".join(meta_parts)}</div>')
        exp_company = exp_role = None

    def flush_edu(date: str = '') -> None:
        nonlocal edu_institution, edu_course
        if edu_course is None:
            return
        html.append(f'<div class="edu-course">{_esc(edu_course)}</div>')
        meta_parts = []
        if edu_institution:
            meta_parts.append(_esc(edu_institution))
        if date:
            meta_parts.append(_esc(date))
        if meta_parts:
            html.append(f'<div class="edu-institution">{" \u2014 ".join(meta_parts)}</div>')
        edu_institution = edu_course = None

    def flush_proj() -> None:
        nonlocal proj_buffer
        if proj_buffer:
            _render_proj_buffer(html, proj_buffer)
            proj_buffer = []

    while i < n:
        stripped = lines[i].rstrip().strip()
        i += 1

        # Blank line
        if not stripped:
            close_ul()
            if current_section in _PROJ_SECTIONS:
                flush_proj()
            # If expecting a date, keep state alive — blank lines between header and date are common
            if not expect_date:
                flush_exp()
                flush_edu()
            continue

        # Section header
        if _is_section(stripped):
            close_ul()
            flush_exp()
            flush_edu()
            flush_proj()
            flush_cert()
            flush_skill()
            expect_date = False
            current_section = stripped.upper()
            html.append(f'<h2>{_esc(stripped)}</h2>')
            continue

        # Bullet
        if stripped.startswith('- ') or stripped.startswith('* '):
            flush_exp()
            expect_date = False
            if not in_ul:
                html.append('<ul>')
                in_ul = True
            html.append(f'<li>{_inline(stripped[2:])}</li>')
            continue

        # PROJECTS — buffer content lines; detect project boundaries by content
        if current_section in _PROJ_SECTIONS:
            close_ul()
            is_url = stripped.startswith('http://') or stripped.startswith('https://')
            has_inline_link = bool(re.search(r'\|\s*https?://', stripped))

            # Em-dash line after buffer already has a link/desc → new project starting
            if proj_buffer and '\u2014' in stripped and not is_url:
                buf_has_end = any(
                    l.startswith('http') or bool(re.search(r'\|\s*https?://', l)) or len(l) > 70
                    for l in proj_buffer
                )
                if buf_has_end:
                    flush_proj()

            proj_buffer.append(stripped)

            # Bare URL or inline link = last line of this project → flush immediately
            if is_url or has_inline_link:
                flush_proj()
            continue

        # Date line — checked BEFORE em-dash so "2025-01 — 2024-06" isn't re-parsed as a header
        if expect_date:
            expect_date = False
            # Collect the current line; peek ahead for a second date-like line (split dates)
            date_text = stripped
            j = i
            while j < n and not lines[j].strip():  # skip blanks
                j += 1
            if j < n:
                nxt = lines[j].strip()
                if (re.search(r'\b(20\d\d|19\d\d|presente|present|atual)\b', nxt, re.IGNORECASE)
                        and not _is_section(nxt) and not nxt.startswith('-')):
                    date_text = date_text + ' \u2013 ' + nxt
                    i = j + 1
            if exp_role is not None:
                flush_exp(date=date_text)
            elif edu_course is not None:
                flush_edu(date=date_text)
            else:
                html.append(f'<div class="date">{_esc(date_text)}</div>')
            continue

        # Em dash line (U+2014 —)
        if '\u2014' in stripped:
            close_ul()
            left, _, right = stripped.partition('\u2014')
            left, right = left.strip(), right.strip()

            if current_section in _EXP_SECTIONS:
                flush_exp()
                # Company — Role
                exp_company, exp_role = left, right
                expect_date = True

            elif current_section in _EDU_SECTIONS:
                flush_edu()
                # Institution — Course
                edu_institution, edu_course = left, right
                expect_date = True

            else:
                flush_exp()
                exp_company, exp_role = left, right
                expect_date = True

            continue

        # Skill section lines
        if current_section in _SKILL_SECTIONS:
            close_ul()
            # One-liner format: "Category: item1, item2, item3"
            if _is_skill_line(stripped):
                flush_skill()
                idx = stripped.index(':')
                key = stripped[:idx].strip()
                val = stripped[idx + 1:].strip()
                html.append(
                    f'<p class="skill-line">'
                    f'<span class="skill-key">{_esc(key)}:</span> {_esc(val)}'
                    f'</p>'
                )
                continue
            # Header-only line: "Category:" (bullet items follow on next lines)
            if stripped.endswith(':') and len(stripped.split()) <= 4:
                flush_skill()
                skill_key = stripped[:-1].strip()
                skill_items = []
                continue
            # Bullet item under a pending skill key: "• item" or "- item"
            if skill_key is not None and (stripped.startswith('•') or stripped.startswith('-') or stripped.startswith('*')):
                item = stripped.lstrip('•-* ').strip()
                if item:
                    skill_items.append(item)
                continue
            # Anything else in skills section → flush pending key, treat as paragraph
            flush_skill()
            html.append(f'<p>{_inline(stripped)}</p>')
            continue

        # Certification — detect by content, not by line position
        # Meta line = has em-dash AND we have a pending cert waiting for it
        # Name line = everything else (flush previous cert first, then buffer new one)
        if current_section in _CERT_SECTIONS:
            close_ul()
            is_meta = pending_cert is not None and '\u2014' in stripped
            if is_meta:
                flush_cert()
                html.append(f'<div class="cert-meta">{_esc(stripped)}</div>')
            else:
                flush_cert()  # flush previous cert without meta if LLM skipped it
                if '|' in stripped:
                    name_part, _, link_part = stripped.partition('|')
                    pending_cert = {'name': name_part.strip(), 'link': link_part.strip()}
                else:
                    pending_cert = {'name': stripped, 'link': None}
            continue

        # Default paragraph
        close_ul()
        html.append(f'<p>{_inline(stripped)}</p>')

    close_ul()
    flush_exp()
    flush_edu()
    flush_proj()
    flush_cert()
    flush_skill()
    return "\n".join(html)


# ── Cover letter parser ────────────────────────────────────────────────────────

def cover_letter_para_html(texto: str) -> str:
    blocks = re.split(r'\n\s*\n', texto.strip())
    html: list = []
    for idx, block in enumerate(blocks):
        content = ' '.join(l.strip() for l in block.split('\n') if l.strip())
        if not content:
            continue
        css_class = ' class="salutation"' if idx == 0 else ''
        html.append(f'<p{css_class}>{_inline(content)}</p>')
    return "\n".join(html)


# ── PDF generators ─────────────────────────────────────────────────────────────

def _wrap_html(body: str, title: str, lang: str = "pt-BR") -> str:
    return (
        f'<!DOCTYPE html>\n'
        f'<html lang="{lang}">\n'
        f'<head><meta charset="UTF-8"><title>{_esc(title)}</title></head>\n'
        f'<body>{body}</body>\n'
        f'</html>'
    )


def gerar_pdf_curriculo(texto: str, caminho_saida: str) -> None:
    corpo = curriculo_para_html(texto)
    HTML(string=_wrap_html(corpo, "Currículo")).write_pdf(
        caminho_saida, stylesheets=[RESUME_CSS]
    )


def gerar_pdf_cover_letter(texto: str, caminho_saida: str) -> None:
    corpo = cover_letter_para_html(texto)
    HTML(string=_wrap_html(corpo, "Carta de Apresentação")).write_pdf(
        caminho_saida, stylesheets=[COVER_CSS]
    )


# ── Entry point ────────────────────────────────────────────────────────────────

def main() -> None:
    dados = json.load(sys.stdin)
    curriculo    = dados.get("curriculo", "")
    cover_letter = dados.get("cover_letter", "")
    output_dir   = dados.get("output_dir", "output")

    os.makedirs(output_dir, exist_ok=True)

    resume_path       = os.path.join(output_dir, "resume.pdf")
    cover_letter_path = os.path.join(output_dir, "cover_letter.pdf")

    gerar_pdf_curriculo(curriculo, resume_path)
    gerar_pdf_cover_letter(cover_letter, cover_letter_path)

    print(json.dumps({"resume_path": resume_path, "cover_letter_path": cover_letter_path}))


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        print(json.dumps({"error": str(e), "traceback": traceback.format_exc()}))
        sys.exit(1)
