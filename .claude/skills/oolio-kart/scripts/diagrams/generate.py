"""Generates docs/diagrams/{order,coupon,auth}.drawio in the repo's simple style:
short labels, few boxes, at most one small note per page, no code.

Run via build.sh (generate + export PNGs). Edit this file rather than the
.drawio files; hand edits in draw.io are overwritten on the next run. Values on
the dry-run pages were captured from the real stack; if behaviour changes,
re-capture them and update the numbers here.
"""
import sys
from drawlib import Page

OUT = sys.argv[1]

def B(p, title, sub="", x=0, y=0, w=210, h=64, color="blue", shape="rounded=1;arcSize=12;"):
    label = f"<b>{title}</b>" + (f"<br><span style='font-size:12px'>{sub}</span>" if sub else "")
    return p.box(label, x, y, w, h, color, shape=shape, extra="fontSize=14;")

def D(p, q, x, y, w=220, h=86):
    return p.box(q, x, y, w, h, "white", shape="rhombus;", extra="fontSize=13;")

def X(p, text, x, y, w=190, h=46):  # early exit
    return p.box(text, x, y, w, h, "red", shape="rounded=1;arcSize=12;", extra="fontSize=13;")

def E(p, a, b, label="", extra=""):
    return p.edge(a, b, label, extra="fontSize=12;" + extra)

def note(p, title, bullets, x, y, w=300, h=None):
    h = h or 40 + 24 * len(bullets)
    body = f"<b>{title}</b><br>" + "<br>".join("• " + b for b in bullets)
    return p.box(body, x, y, w, h, "yellow", shape="shape=note;size=14;",
                 extra="align=left;verticalAlign=top;spacingLeft=10;spacingTop=8;fontSize=13;")

def header(p, title, sub):
    p.text(f"<b>{title}</b>", 30, 20, 1300, 32, size=24)
    p.text(sub, 30, 56, 1300, 24, size=14)

CYL = "shape=cylinder3;boundedLbl=1;backgroundOutline=1;size=12;"
DOC = "shape=document;boundedLbl=1;size=0.18;"

# ====================================================================== ORDER
oh = Page("HLD - Order placement", "oh")
header(oh, "Order placement", "POST /order: price on the server, check the coupon, save in one transaction.")
cl = B(oh, "Client", "web app · Postman · curl", 30, 130, 180, 64, "gray")
mw = B(oh, "Middleware", "request id · metrics · logs · CORS", 250, 130, 230)
au = B(oh, "Auth", "Bearer JWT or api_key", 520, 130, 190, 64, "orange")
hd = B(oh, "Order handler", "JSON in, JSON out", 750, 130, 190)
sv = B(oh, "Order service", "validate · merge · price", 980, 130, 210, 64, "purple")
E(oh, cl, mw, "HTTP"); E(oh, mw, au); E(oh, au, hd); E(oh, hd, sv)
pr = B(oh, "Products", "1 query for the cart", 830, 300, 190, 64, "purple")
rp = B(oh, "Order repository", "1 transaction", 1060, 300, 190, 64, "green")
cp = B(oh, "Coupons", "in-memory index", 1290, 300, 190, 64, "green")
E(oh, sv, pr); E(oh, sv, rp)
E(oh, sv, cp, "", extra="exitX=1;exitY=0.5;entryX=0.5;entryY=0;")
pg = B(oh, "Postgres", "", 935, 450, 200, 80, "blue", CYL)
E(oh, pr, pg, "", extra="exitX=0.5;exitY=1;entryX=0.25;entryY=0;")
E(oh, rp, pg, "", extra="exitX=0.5;exitY=1;entryX=0.75;entryY=0;")
note(oh, "Key points", ["Prices come from the catalog, never the client",
                        "5 SQL statements per order, any cart size",
                        "Errors: 400 · 401 · 403 · 422 · 500"], 250, 300, 420)

ol = Page("LLD - Order placement flow", "ol")
header(ol, "Order placement: step by step", "Top to bottom; red boxes are the errors a client can get.")
CX, EXX = 300, 640
y = 110
steps = []
def lstep(node):
    if steps:
        E(ol, steps[-1][0], node, steps[-1][1])
    steps.append((node, ""))
def ldecide(q, err, yes_continues=True):
    global y
    d = D(ol, q, CX - 10, y)
    lstep(d)
    e = X(ol, err, EXX, y + 20)
    E(ol, d, e, "no" if yes_continues else "yes", extra="exitX=1;exitY=0.5;entryX=0;entryY=0.5;")
    steps[-1] = (d, "yes" if yes_continues else "no")
    y += 120
def lbox(t, s="", color="purple"):
    global y
    n = B(ol, t, s, CX, y, 200, 60, color)
    lstep(n)
    y += 95
lbox("Middleware", "request id · metrics · logs · CORS", "blue")
ldecide("Credentials valid?", "401 missing or bad token<br>403 wrong api_key")
ldecide("Allowed to order?", "403 missing scope")
ldecide("Valid JSON?", "400 bad JSON")
ldecide("Items OK?<br><span style='font-size:11px'>not empty · qty 1 to 1000</span>", "422 invalid items")
lbox("Merge duplicates", "same product → one line")
lbox("Look up products", "1 query for the whole cart")
ldecide("All products exist?", "422 lists every<br>unknown product")
ldecide("Coupon valid?<br><span style='font-size:11px'>if one was sent</span>", "422 invalid coupon")
lbox("Price the order", "subtotal · 5% discount · total")
lbox("Save", "1 transaction: order + all lines", "green")
lbox("200 OK", "order JSON", "green")
note(ol, "Why it is fast", ["1 query reads every product", "1 insert writes every line",
                            "5 statements for any cart size (was 23 for 10 products)"], 900, 110, 380)

od = Page("Dry run - real request", "od")
header(od, "Order placement: dry run", "A real request on Docker + Postgres, with the values it produced.")
B(od, "Request", "HAPPYHRS · product 1 × 2, 3 × 1, 1 × 1", 30, 110, 330, 64, "gray")
rows = [("1 · Merge", "product 1 × 3, product 3 × 1"),
        ("2 · Prices", "6.50 × 3 + 8.00 × 1 = 27.50"),
        ("3 · Coupon", "HAPPYHRS valid → 5% = 1.38"),
        ("4 · Total", "27.50 − 1.38 = 26.12"),
        ("5 · Save", "BEGIN · order · 2 lines in 1 insert · COMMIT"),
        ("6 · Response", "200 · id 5cb5a242-…")]
prev = None
yy = 210
for i, (t, v) in enumerate(rows):
    n = B(od, t, v, 30, yy, 330, 60, "green" if i >= 4 else "purple")
    if prev:
        E(od, prev, n)
    prev = n
    yy += 88
table = ("<b>Error responses</b> (same stack)<br><br>"
         "<table cellpadding='5' style='font-size:13px'>"
         "<tr><td><b>400</b></td><td>body is not valid JSON</td></tr>"
         "<tr><td><b>401</b></td><td>no credentials or bad token</td></tr>"
         "<tr><td><b>403</b></td><td>wrong api_key</td></tr>"
         "<tr><td><b>422</b></td><td>unknown product · bad coupon · qty over 1000</td></tr>"
         "<tr><td><b>405</b></td><td>GET /order</td></tr></table>")
od.box(table, 440, 110, 420, 250, "white", extra="align=left;verticalAlign=top;spacingLeft=12;spacingTop=10;")
note(od, "SQL per order", ["5 statements, for any cart size", "10 products: 23 → 5",
                           "p50 5.7 → 2.3 ms (10-product orders)"], 440, 400, 420)

# ====================================================================== COUPON
ch = Page("HLD - Coupon validation", "ch")
header(ch, "Coupon validation", "A code is valid if it is 8 to 10 characters and appears in at least 2 of the 3 files.")
ch.zone("Offline, once", 20, 100, 1360, 190, "gray")
s3 = B(ch, "S3", "3 gzip files", 50, 160, 150, 80, "orange", CYL)
fe = B(ch, "fetch_coupons.sh", "resumable download", 250, 168, 200)
raw = B(ch, "coupons/raw", "2.1 GB · 313M lines", 500, 160, 200, 80, "gray", "shape=datastore;")
bi = B(ch, "buildindex", "filter · sort · merge", 750, 168, 200)
idx = B(ch, "coupons.idx", "96 bytes · 8 codes", 1000, 160, 200, 80, "green", DOC)
E(ch, s3, fe); E(ch, fe, raw); E(ch, raw, bi); E(ch, bi, idx)
ch.zone("Runtime, every order", 20, 320, 1360, 250, "blue")
boot = B(ch, "Server startup", "loads the index", 1000, 370, 200)
ix = B(ch, "Index in memory", "binary search", 750, 370, 200, 64, "green")
os_ = B(ch, "Order service", "IsValid(code)", 500, 370, 200, 64, "purple")
E(ch, idx, boot, "", extra="exitX=0.5;exitY=1;entryX=0.5;entryY=0;")
E(ch, boot, ix); E(ch, os_, ix, "check code")
m1 = X(ch, "file missing → every coupon rejected", 1080, 480, 260, 50)
m2 = X(ch, "file corrupt → server stops", 790, 480, 260, 50)
E(ch, boot, m1, "", extra="dashed=1;exitX=0.75;exitY=1;entryX=0.5;entryY=0;")
E(ch, boot, m2, "", extra="dashed=1;exitX=0.25;exitY=1;entryX=0.5;entryY=0;")

cm = Page("Package map - interfaces", "cm")
header(cm, "Coupon package: interfaces", "Three small interfaces keep each part replaceable and testable.")
IMPL = "endArrow=block;endFill=0;dashed=1;endSize=12;"
c1 = B(cm, "buildindex", "command", 60, 110, 200, 56, "gray")
c2 = B(cm, "server", "startup", 520, 110, 200, 56, "gray")
c3 = B(cm, "order service", "", 980, 110, 200, 56, "gray")
b1 = B(cm, "Build", "reads sources, writes index", 60, 230, 200)
b2 = B(cm, "LoadIndex", "reads the file", 520, 230, 200)
src = B(cm, "Source", "interface: where codes come from", 20, 370, 210, 64, "white")
wr = B(cm, "io.Writer", "interface: where the index goes", 270, 370, 210, 64, "white")
va = B(cm, "Validator", "interface: is this code valid?", 980, 230, 200, 64, "white")
g1 = B(cm, "GzipFileSource", "real files", 0, 510, 130, 56, "green")
g2 = B(cm, "fakeSource", "tests", 145, 510, 110, 56, "gray")
v1 = B(cm, "Index", "sorted keys", 900, 370, 150, 56, "green")
v2 = B(cm, "Unavailable", "rejects all", 1110, 370, 150, 56, "orange")
E(cm, c1, b1); E(cm, c2, b2); E(cm, c3, va, "uses")
E(cm, b1, src, "Scan", extra="exitX=0.3;exitY=1;entryX=0.5;entryY=0;")
E(cm, b1, wr, "write", extra="exitX=0.8;exitY=1;entryX=0.5;entryY=0;")
E(cm, b2, v1, "returns", extra="exitX=1;exitY=0.5;entryX=0.5;entryY=0;")
for a, b in ((g1, src), (g2, src), (v1, va), (v2, va)):
    E(cm, a, b, "implements", extra=IMPL)

cl_ = Page("LLD - BuildIndex internals", "cl")
header(cl_, "Coupon index build: step by step", "Each file is processed in parallel, then the results are merged.")
cl_.zone("For each of the 3 files, in parallel", 20, 100, 700, 470, "purple")
st = [("Read", "gzip, one line at a time"), ("Filter", "keep 8 to 10 characters"),
      ("Encode", "11-byte key: length + code"), ("Sort + dedupe", "one list per file")]
prev = None
yy = 150
for t, s in st:
    n = B(cl_, t, s, 250, yy, 230, 64)
    if prev:
        E(cl_, prev, n)
    prev = n
    yy += 100
mg = B(cl_, "Merge", "keep codes in 2+ files", 820, 250, 220, 64, "green")
wf = B(cl_, "Write coupons.idx", "header + sorted keys", 820, 400, 220, 64, "green")
E(cl_, prev, mg, "", extra="exitX=1;exitY=0.5;entryX=0;entryY=0.5;"); E(cl_, mg, wf)
note(cl_, "Numbers", ["313M lines → 8 valid codes", "~5 minutes, ~1.2 GB per file",
                      "Lookup: length check + binary search"], 1080, 250, 320)

cd = Page("LLD - Dry run", "cd")
header(cd, "Coupon index build: dry run", "13 lines in 3 small files, traced to the final index (bytes verified).")
files = [("File 1", "HAPPYHRS · SHORT · BURGER10<br>FIFTYOFF · BURGER10 · ABCDEFGHIJK"),
         ("File 2", "FIFTYOFF · PIZZA2024<br>HAPPYHRS · TACOTUES"),
         ("File 3", "HAPPYHRS · TACOTUES · SUPER100")]
for i, (t, s) in enumerate(files):
    B(cd, t, s, 30, 110 + i * 100, 330, 76, "gray")
count = ("<b>How many files each code is in</b><br><br>"
         "<table cellpadding='4' style='font-size:13px'>"
         "<tr><td>HAPPYHRS</td><td>3</td><td style='color:#2e7d32'>✓ valid</td></tr>"
         "<tr><td>FIFTYOFF</td><td>2</td><td style='color:#2e7d32'>✓ valid</td></tr>"
         "<tr><td>TACOTUES</td><td>2</td><td style='color:#2e7d32'>✓ valid</td></tr>"
         "<tr><td>BURGER10</td><td>1</td><td style='color:#b71c1c'>✗ (twice, same file)</td></tr>"
         "<tr><td>PIZZA2024 · SUPER100</td><td>1</td><td style='color:#b71c1c'>✗</td></tr>"
         "<tr><td>SHORT · ABCDEFGHIJK</td><td>–</td><td style='color:#b71c1c'>✗ wrong length</td></tr></table>")
ct = cd.box(count, 430, 110, 400, 250, "white", extra="align=left;verticalAlign=top;spacingLeft=12;spacingTop=10;")
out = B(cd, "coupons.idx", "41 bytes = 8 header + 3 × 11", 900, 180, 250, 80, "green", DOC)
E(cd, ct, out)
look = ("<b>Lookups</b><br><br><table cellpadding='4' style='font-size:13px'>"
        "<tr><td>TACOTUES</td><td style='color:#2e7d32'>valid</td></tr>"
        "<tr><td>PIZZA2024</td><td style='color:#b71c1c'>invalid</td></tr>"
        "<tr><td>SHORT</td><td style='color:#b71c1c'>rejected by length</td></tr></table>")
cd.box(look, 900, 300, 250, 150, "white", extra="align=left;verticalAlign=top;spacingLeft=12;spacingTop=10;")

# ====================================================================== AUTH
ah = Page("HLD - JWT auth", "ah")
header(ah, "Authentication", "Get a token once, then send it with every order. The api_key still works.")
cli = B(ah, "Client", "", 30, 200, 150, 64, "gray")
tk = B(ah, "POST /auth/token", "username + password", 260, 110, 220, 64, "orange")
us = B(ah, "User store", "bcrypt password check", 540, 110, 210, 64, "purple")
jw = B(ah, "JWT manager", "signs HS256 tokens, 15 min", 810, 110, 230, 64, "purple")
E(ah, cli, tk, "① login", extra="exitX=0.5;exitY=0;entryX=0;entryY=0.5;"); E(ah, tk, us); E(ah, us, jw, "issue")
po = B(ah, "POST /order", "Bearer token or api_key", 260, 300, 220, 64, "orange")
an = B(ah, "Authenticate", "who is calling?", 540, 300, 210, 64)
sc = B(ah, "Require scope", "may they order?", 810, 300, 230, 64)
oh_ = B(ah, "Order handler", "", 1100, 300, 180, 64, "green")
E(ah, cli, po, "② order", extra="exitX=0.5;exitY=1;entryX=0;entryY=0.5;"); E(ah, po, an); E(ah, an, sc); E(ah, sc, oh_)
E(ah, an, jw, "verify token", extra="dashed=1;exitX=0.5;exitY=0;entryX=0.25;entryY=1;")
note(ah, "401 vs 403", ["401: we don't know who you are", "403: we know you, but not allowed",
                        "Every bad token gets the same 401"], 540, 430, 380)

al = Page("LLD - token issue and verify", "al")
header(al, "Authentication: step by step", "Left: getting a token. Right: checking it on POST /order.")
def chain(p, x, items, exit_x):
    prev = None
    y = 110
    for kind, *a in items:
        if kind == "box":
            n = B(p, a[0], a[1], x, y, 210, 60, a[2]); y += 95
        else:
            n = D(p, a[0], x - 5, y, 220, 84)
            e = X(p, a[1], exit_x, y + 19, 180, 46)
            E(p, n, e, "no", extra="exitX=1;exitY=0.5;entryX=0;entryY=0.5;")
            y += 115
        if prev:
            E(p, prev, n, "yes" if prev_kind == "dec" else "")
        prev, prev_kind = n, kind
    return prev
chain(al, 80, [("box", "POST /auth/token", "", "orange"),
               ("dec", "Valid JSON?", "400 bad JSON"),
               ("dec", "Password correct?", "401 invalid login"),
               ("box", "Sign token", "HS256 · 15 minutes", "purple"),
               ("box", "200", "token JSON", "green")], 330)
chain(al, 640, [("box", "POST /order", "", "orange"),
                ("dec", "Credentials sent?", "401 missing"),
                ("dec", "Token or key valid?", "401 bad token<br>403 wrong key"),
                ("dec", "Has create_order?", "403 not allowed"),
                ("box", "Order handler", "", "green")], 890)

ad = Page("Dry run - real requests", "ad")
header(ad, "Authentication: dry run", "Real requests on Docker + Postgres, with the values they produced.")
steps_ = [("1 · Login", "demo / demo1234 → 200 in 99.6 ms"),
          ("2 · Token", "HS256, 295 characters, valid 15 min"),
          ("3 · Order with token", "signature, issuer, audience, expiry ✓"),
          ("4 · Result", "200 in 12.4 ms · total 13.30")]
prev = None
yy = 110
for i, (t, v) in enumerate(steps_):
    n = B(ad, t, v, 30, yy, 360, 60, "green" if i == 3 else "purple")
    if prev:
        E(ad, prev, n)
    prev = n
    yy += 90
errs = ("<b>Error responses</b><br><br><table cellpadding='5' style='font-size:13px'>"
        "<tr><td><b>401</b></td><td>wrong password or unknown user</td></tr>"
        "<tr><td><b>400</b></td><td>login body is not valid JSON</td></tr>"
        "<tr><td><b>401</b></td><td>tampered or expired token</td></tr>"
        "<tr><td><b>401</b></td><td>no credentials</td></tr>"
        "<tr><td><b>403</b></td><td>wrong api_key</td></tr></table>")
ad.box(errs, 470, 110, 400, 230, "white", extra="align=left;verticalAlign=top;spacingLeft=12;spacingTop=10;")
note(ad, "Same timing on purpose", ["Wrong password: ~62 ms", "Unknown user: ~62 ms",
                                    "So usernames can't be guessed"], 470, 380, 400)

def write(path, pages):
    xml = '<mxfile host="app.diagrams.net">' + "".join(p.xml(w, h) for p, w, h in pages) + "</mxfile>"
    open(path, "w").write(xml)

write(f"{OUT}/order.drawio", [(oh, 1520, 580), (ol, 1320, 1500), (od, 900, 760)])
write(f"{OUT}/coupon.drawio", [(ch, 1400, 600), (cm, 1300, 600), (cl_, 1420, 600), (cd, 1200, 520)])
write(f"{OUT}/auth.drawio", [(ah, 1320, 560), (al, 1120, 700), (ad, 900, 540)])
print("ok")
