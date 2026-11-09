#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Build owned PNG icons into ICNS, ICO and an AMD64 COFF resource object.

No compiler or third-party packages needed. Resource structures follow:
https://learn.microsoft.com/en-us/windows/win32/debug/pe-format
https://devblogs.microsoft.com/oldnewthing/20231025-00/?p=108925
Use --check to verify committed generated files without writing.
"""
import pathlib, struct, sys
ROOT = pathlib.Path(__file__).resolve().parents[1]
P = ROOT / 'assets/icons'
pack = struct.pack

def png(size):
    data = (P / f'png/{size}.png').read_bytes()
    assert data[:8] == b'\x89PNG\r\n\x1a\n'
    assert struct.unpack('>II', data[16:24]) == (size, size)
    return data

def resource_object(images):
    group = pack('<HHH', 0, 1, len(images))
    for ident, (size, data) in enumerate(images, 1):
        group += pack('<BBBBHHIH', size % 256, size % 256, 0, 0, 1, 32, len(data), ident)
    tree = {3: {i: {1033: data} for i, (_, data) in enumerate(images, 1)}, 14: {1: {1033: group}}}
    section = bytearray()
    leaves = []
    def directory(nodes):
        offset = len(section)
        section.extend(pack('<IIHHHH', 0, 0, 0, 0, 0, len(nodes)))
        section.extend(b'\0' * (8 * len(nodes)))
        for index, (ident, node) in enumerate(sorted(nodes.items())):
            if isinstance(node, dict):
                target = directory(node) | 0x80000000
            else:
                target = len(section)
                section.extend(b'\0' * 16)
                leaves.append((target, node))
            struct.pack_into('<II', section, offset + 16 + index * 8, ident, target)
        return offset
    directory(tree)
    relocations = bytearray()
    for entry, data in leaves:
        section.extend(b'\0' * (-len(section) % 4))
        offset = len(section)
        section.extend(data)
        struct.pack_into('<IIII', section, entry, offset, len(data), 0, 0)
        relocations.extend(pack('<IIH', entry, 0, 3))  # IMAGE_REL_AMD64_ADDR32NB
    reloc_offset = 60 + len(section)
    sym_offset = reloc_offset + len(relocations)
    header = pack('<HHIIIHH', 0x8664, 1, 0, sym_offset, 1, 0, 0)
    sh = pack('<8sIIIIIIHHI', b'.rsrc\0\0\0', 0, 0, len(section), 60, reloc_offset, 0, len(leaves), 0, 0x40000040)
    symbol = pack('<8sIhHBB', b'.rsrc\0\0\0', 0, 1, 0, 3, 0)
    return header + sh + section + relocations + symbol + pack('<I', 4)

def outputs():
    yield ROOT / 'internal/companion/web/icon.svg', (P / 'dmz.svg').read_bytes()
    for size in (16,32,48,128):
        yield ROOT / f'apps/dmz/public/icons/{size}.png', png(size)
    chunks = b''
    for typ, size in [('icp4',16),('icp5',32),('icp6',64),('ic07',128),('ic08',256),('ic09',512),('ic10',1024),('ic11',32),('ic12',64),('ic13',256),('ic14',512)]:
        data = png(size)
        chunks += typ.encode() + pack('>I', 8 + len(data)) + data
    yield P / 'DMZ.icns', b'icns' + pack('>I', 8 + len(chunks)) + chunks
    images = [(size, png(size)) for size in (16,32,48,64,128,256)]
    offset = 6 + 16 * len(images)
    ico = pack('<HHH', 0, 1, len(images))
    for size, data in images:
        ico += pack('<BBBBHHII', size % 256, size % 256, 0, 0, 1, 32, len(data), offset)
        offset += len(data)
    yield P / 'DMZ.ico', ico + b''.join(data for _, data in images)
    yield ROOT / 'cmd/dmz/dmz_icon_windows_amd64.syso', resource_object(images)

if __name__ == '__main__':
    for path, data in outputs():
        if '--check' in sys.argv:
            if not path.exists() or path.read_bytes() != data:
                raise SystemExit('icon_resource_out_of_date: ' + str(path.relative_to(ROOT)))
        else:
            path.write_bytes(data)
    print('PASS icon resources match committed PNG assets')
