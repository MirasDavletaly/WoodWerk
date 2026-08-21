# -*- coding: utf-8 -*-
"""Собирает kk.json и en.json из троек «русский — қазақша — English».

Сопоставление идёт по русскому тексту, а не по ключу: ключи — это хеши
исходной строки, они меняются при любой правке текста, а сам текст читаем
и стабилен.
"""
import io
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import tr_part1
import tr_part2
import tr_part3
import tr_part4

ROWS = tr_part1.ROWS + tr_part2.ROWS + tr_part3.ROWS + tr_part4.ROWS


def norm(text):
    """Пробелы и переводы строк для сравнения не важны."""
    return ' '.join(text.split())


def main():
    ru = json.load(io.open('assets/i18n/ru.json', encoding='utf-8'))

    # русский текст -> ключ
    by_text = {}
    for key, value in ru.items():
        by_text.setdefault(norm(value), key)

    kk, en = {}, {}
    unmatched = []
    duplicates = []
    seen = set()

    for row in ROWS:
        source, kaz, eng = row
        n = norm(source)
        if n in seen:
            duplicates.append(source[:60])
        seen.add(n)

        key = by_text.get(n)
        if not key:
            unmatched.append(source[:70])
            continue
        kk[key] = kaz
        en[key] = eng

    missing = [(k, ru[k]) for k in ru if k not in kk]

    print('  строк в исходнике:      %d' % len(ru))
    print('  переведено:             %d' % len(kk))
    print('  перевод не подошёл:     %d' % len(unmatched))
    print('  дубликаты в переводах:  %d' % len(duplicates))
    print('  осталось без перевода:  %d' % len(missing))

    if unmatched:
        print('\n  НЕ НАШЛИСЬ В ИСХОДНИКЕ (текст изменился?):')
        for t in unmatched[:20]:
            print('   ', t)

    if missing:
        print('\n  БЕЗ ПЕРЕВОДА:')
        for k, v in sorted(missing, key=lambda kv: kv[1])[:80]:
            print('    %s\t%s' % (k, norm(v)[:100]))

    if len(sys.argv) > 1 and sys.argv[1] == 'write':
        for code, data in (('kk', kk), ('en', en)):
            path = os.path.join('assets', 'i18n', code + '.json')
            io.open(path, 'w', encoding='utf-8', newline='\n').write(
                json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True))
            print('\n  записан:', path, '(%d строк)' % len(data))


if __name__ == '__main__':
    main()
