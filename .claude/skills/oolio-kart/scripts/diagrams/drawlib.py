"""Tiny helper for writing draw.io (mxGraph) XML: boxes, zones, text, edges."""
import base64, sys, urllib.parse, zlib
from xml.sax.saxutils import escape

C = {
    "blue": ("#dae8fc", "#6c8ebf"),
    "green": ("#d5e8d4", "#82b366"),
    "yellow": ("#fff2cc", "#d6b656"),
    "orange": ("#ffe6cc", "#d79b00"),
    "red": ("#f8cecc", "#b85450"),
    "purple": ("#e1d5e7", "#9673a6"),
    "gray": ("#f5f5f5", "#666666"),
    "white": ("#ffffff", "#666666"),
}


class Page:
    def __init__(self, name, pid):
        self.name, self.pid, self.cells, self.n = name, pid, [], 0

    def _id(self):
        self.n += 1
        return f"{self.pid}{self.n}"

    def box(self, label, x, y, w, h, color="blue", shape="rounded=1;arcSize=8;", extra=""):
        fill, stroke = C[color]
        cid = self._id()
        style = f"{shape}whiteSpace=wrap;html=1;fillColor={fill};strokeColor={stroke};fontSize=12;{extra}"
        self.cells.append(
            f'<mxCell id="{cid}" value="{escape(label, {chr(34): "&quot;"})}" style="{style}" vertex="1" parent="1">'
            f'<mxGeometry x="{x}" y="{y}" width="{w}" height="{h}" as="geometry"/></mxCell>'
        )
        return cid

    def zone(self, label, x, y, w, h, color="gray"):
        fill, stroke = C[color]
        return self.box(label, x, y, w, h, color,
                        shape="rounded=1;arcSize=2;dashed=1;",
                        extra=f"verticalAlign=top;align=left;spacingLeft=12;spacingTop=6;fontStyle=1;fontSize=14;fontColor={stroke};")

    def text(self, label, x, y, w, h, size=12, extra=""):
        cid = self._id()
        self.cells.append(
            f'<mxCell id="{cid}" value="{escape(label, {chr(34): "&quot;"})}" style="text;html=1;whiteSpace=wrap;align=left;verticalAlign=top;fontSize={size};{extra}" vertex="1" parent="1">'
            f'<mxGeometry x="{x}" y="{y}" width="{w}" height="{h}" as="geometry"/></mxCell>'
        )
        return cid

    def edge(self, s, t, label="", dashed=False, extra=""):
        cid = self._id()
        style = ("edgeStyle=orthogonalEdgeStyle;rounded=1;html=1;endArrow=block;endFill=1;"
                 "strokeColor=#444444;fontSize=11;labelBackgroundColor=#ffffff;"
                 + ("dashed=1;" if dashed else "") + extra)
        self.cells.append(
            f'<mxCell id="{cid}" value="{escape(label, {chr(34): "&quot;"})}" style="{style}" edge="1" parent="1" source="{s}" target="{t}">'
            f'<mxGeometry relative="1" as="geometry"/></mxCell>'
        )
        return cid

    def xml(self, w, h):
        return (f'<diagram name="{self.name}" id="{self.pid}">'
                f'<mxGraphModel dx="{w}" dy="{h}" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="{w}" pageHeight="{h}" math="0" shadow="0">'
                f'<root><mxCell id="0"/><mxCell id="1" parent="0"/>{"".join(self.cells)}</root></mxGraphModel></diagram>')


MONO = "fontFamily=Courier New;align=left;spacingLeft=10;"



def viewer(page_xml):
    raw = urllib.parse.quote(f'<mxfile>{page_xml}</mxfile>', safe="~()*!.'")
    comp = zlib.compressobj(9, zlib.DEFLATED, -15)
    data = comp.compress(raw.encode()) + comp.flush()
    return "https://viewer.diagrams.net/?lightbox=1&nav=1#R" + urllib.parse.quote(base64.b64encode(data).decode(), safe="")


def note(page, label, x, y, w, h, color="yellow"):
    return page.box(label, x, y, w, h, color, shape="shape=note;size=14;",
                    extra="align=left;verticalAlign=top;spacingLeft=8;spacingTop=4;")
