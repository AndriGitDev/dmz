#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Verify source-derived icons and the actual linked Windows icon resources."""
import importlib.util, pathlib, struct, sys
ROOT = pathlib.Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('dmz_icon_builder', ROOT/'scripts/build-icon-resources.py')
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)
for path, expected in builder.outputs():
    assert path.read_bytes() == expected, 'generated icon resource differs: ' + str(path.relative_to(ROOT))
if len(sys.argv) > 1:
    data = pathlib.Path(sys.argv[1]).read_bytes()
    pe = struct.unpack_from('<I',data,60)[0]
    assert data[pe:pe+4] == b'PE\0\0'
    count = struct.unpack_from('<H',data,pe+6)[0]
    optional = struct.unpack_from('<H',data,pe+20)[0]
    table = pe+24+optional
    resource = None
    for i in range(count):
        sh = table+40*i
        if data[sh:sh+8].rstrip(b'\0') == b'.rsrc':
            size, va, rawsize, raw = struct.unpack_from('<IIII',data,sh+8)
            resource = data[raw:raw+rawsize]
            assert 0 < size <= rawsize
    assert resource is not None, 'executable has no .rsrc'
    def entries(offset):
        names, ids = struct.unpack_from('<HH',resource,offset+12)
        assert names == 0
        return dict(struct.unpack_from('<II',resource,offset+16+8*i) for i in range(ids))
    root = entries(0)
    assert set(root) == {3,14}, 'missing icon or icon group'
    for typ, branch in root.items():
        assert branch & 0x80000000
        icons = entries(branch & 0x7fffffff)
        assert len(icons) == (6 if typ == 3 else 1)
        for ident, language in icons.items():
            assert language & 0x80000000
            leaf = entries(language & 0x7fffffff)
            assert set(leaf) == {1033}
            offset, length, codepage, reserved = struct.unpack_from('<IIII',resource,leaf[1033])
            blob = resource[offset-va:offset-va+length]
            assert len(blob) == length and reserved == 0
            if typ == 3:
                assert blob == builder.png((16,32,48,64,128,256)[ident-1]), 'linked icon bytes differ'
            else:
                assert struct.unpack_from('<HHH',blob) == (0,1,6)
    print('PASS linked Windows executable contains six exact icon images and an icon group')
print('PASS committed ICNS, ICO, extension icons and COFF match source PNGs')
