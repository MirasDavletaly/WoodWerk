# -*- coding: utf-8 -*-
"""Размечает HTML сайта для перевода и собирает словарь русских строк.

Работает в двух режимах:

    python i18n_extract.py check   — пересобирает страницы без изменений
                                     и проверяет побайтовое совпадение
    python i18n_extract.py apply   — расставляет data-i18n и пишет ru.json

Переводим целые блоки вместе с внутренними тегами (<b>, <a>, <br>), а не
отдельные куски: порядок слов в казахском и английском другой, и разбитая
на части фраза перевелась бы криво.
"""
import hashlib
import io
import json
import os
import re
import sys
from html.parser import HTMLParser

PAGES = ['index.html', 'catalog.html', 'about.html', 'delivery.html',
         'partnership.html', 'contacts.html', 'privacy.html', 'sitemap.html',
         'product.html']

# Содержимое этих тегов не трогаем.
SKIP = {'script', 'style', 'svg'}

# Пустые теги — закрывающего у них нет.
VOID = {'area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input',
        'link', 'meta', 'param', 'source', 'track', 'wbr'}

# Теги, которые считаем частью фразы, а не отдельным блоком.
INLINE = {'b', 'i', 'em', 'strong', 'small', 'span', 'br', 'sup', 'sub',
          'u', 's', 'mark', 'code', 'a', 'nobr'}

# Атрибуты с человеческим текстом.
TEXT_ATTRS = ('placeholder', 'title', 'alt', 'aria-label')

HAS_LETTER = re.compile(r'[А-Яа-яЁёA-Za-z]')
STALE_MARK = re.compile(r'\s+data-i18n(?:-[a-z-]+)?="[^"]*"')


def key_of(text):
    """Ключ — от самого текста, поэтому одинаковые фразы получают один ключ."""
    digest = hashlib.sha1(text.encode('utf-8')).hexdigest()
    return 't' + digest[:10]


class Node(object):
    __slots__ = ('kind', 'raw', 'tag', 'attrs', 'children', 'parent')

    def __init__(self, kind, raw='', tag=None, attrs=None, parent=None):
        self.kind = kind          # root | element | text | other
        self.raw = raw            # исходный текст фрагмента
        self.tag = tag
        self.attrs = attrs or []
        self.children = []
        self.parent = parent


class Builder(HTMLParser):
    """Собирает дерево, сохраняя исходный текст каждого фрагмента."""

    def __init__(self):
        HTMLParser.__init__(self, convert_charrefs=False)
        self.root = Node('root')
        self.stack = [self.root]

    # --- служебное

    def _add(self, node):
        self.stack[-1].children.append(node)
        node.parent = self.stack[-1]
        return node

    def _raw(self):
        return self.get_starttag_text()

    # --- разбор

    def handle_starttag(self, tag, attrs):
        node = self._add(Node('element', self._raw(), tag, attrs))
        if tag not in VOID:
            self.stack.append(node)

    def handle_startendtag(self, tag, attrs):
        self._add(Node('element', self._raw(), tag, attrs))

    def handle_endtag(self, tag):
        # Ищем ближайший открытый тег с таким именем.
        for i in range(len(self.stack) - 1, 0, -1):
            if self.stack[i].tag == tag:
                closed = self.stack[i]
                del self.stack[i:]
                closed.children.append(Node('close', '</%s>' % tag))
                return
        # Закрывающий без открывающего — оставляем как есть.
        self._add(Node('other', '</%s>' % tag))

    def handle_data(self, data):
        self._add(Node('text', data))

    def handle_comment(self, data):
        self._add(Node('other', '<!--%s-->' % data))

    def handle_decl(self, decl):
        self._add(Node('other', '<!%s>' % decl))

    def handle_pi(self, data):
        self._add(Node('other', '<?%s>' % data))

    def handle_entityref(self, name):
        self._add(Node('text', '&%s;' % name))

    def handle_charref(self, name):
        self._add(Node('text', '&#%s;' % name))

    def unknown_decl(self, data):
        self._add(Node('other', '<![%s]>' % data))


def serialize(node):
    """Собирает исходный HTML обратно."""
    out = []
    if node.kind in ('text', 'other', 'close'):
        return node.raw
    if node.kind == 'element':
        out.append(node.raw)
    for child in node.children:
        out.append(serialize(child))
    return ''.join(out)


def inner_html(node):
    """HTML внутри элемента, без самого элемента и его закрывающего тега."""
    return ''.join(serialize(c) for c in node.children if c.kind != 'close')


def is_translatable_block(node):
    """Элемент переводим целиком, если внутри текст и только строчные теги."""
    has_text = False
    for child in node.children:
        if child.kind == 'text' and HAS_LETTER.search(child.raw):
            has_text = True
        elif child.kind == 'element':
            if child.tag in SKIP or child.tag not in INLINE:
                return False
    return has_text


def collect(node, strings, marks, skip_depth=0):
    """Обходит дерево, отмечая, какие элементы переводить."""
    if node.kind == 'element':
        if node.tag in SKIP:
            return
        # атрибуты
        for name, value in node.attrs:
            if value and name in TEXT_ATTRS and HAS_LETTER.search(value):
                text = value.strip()
                if text:
                    k = key_of(text)
                    strings[k] = text
                    marks.setdefault(id(node), {})['attr:' + name] = k

        # У <meta> переводим только описания, а не viewport и не CSP.
        if node.tag == 'meta':
            attrs = dict(node.attrs)
            kind = (attrs.get('name') or attrs.get('property') or '').lower()
            if kind in ('description', 'og:title', 'og:description'):
                text = (attrs.get('content') or '').strip()
                if text:
                    k = key_of(text)
                    strings[k] = text
                    marks.setdefault(id(node), {})['attr:content'] = k

    if node.kind == 'element':
        if any(name == 'data-i18n-ignore' for name, _ in node.attrs):
            return   # внутрь не идём: содержимое одинаково на всех языках

    if node.kind == 'element' and is_translatable_block(node):
        html = inner_html(node).strip()
        if html and HAS_LETTER.search(html):
            k = key_of(html)
            strings[k] = html
            marks.setdefault(id(node), {})['html'] = k
        return  # внутрь не идём: блок переводится целиком

    for child in node.children:
        if child.kind == 'text' and HAS_LETTER.search(child.raw):
            text = child.raw.strip()
            if text:
                k = key_of(text)
                strings[k] = text
                marks[id(child)] = {'wrap': k}
        collect(child, strings, marks)


def rebuild(node, marks):
    """Собирает HTML заново, добавляя data-i18n там, где нужно."""
    if node.kind == 'text':
        mark = marks.get(id(node))
        if mark and 'wrap' in mark:
            raw = node.raw
            body = raw.strip()
            head = raw[:len(raw) - len(raw.lstrip())]
            tail = raw[len(raw.rstrip()):]
            return '%s<span data-i18n="%s">%s</span>%s' % (head, mark['wrap'], body, tail)
        return node.raw
    if node.kind in ('other', 'close'):
        return node.raw

    parts = []
    mark = marks.get(id(node))

    if node.kind == 'element':
        # Прошлую разметку снимаем, чтобы повторный запуск не задваивал атрибуты.
        raw = STALE_MARK.sub('', node.raw)
        if mark:
            extra = []
            for what, k in sorted(mark.items()):
                if what == 'html':
                    extra.append('data-i18n="%s"' % k)
                else:
                    extra.append('data-i18n-%s="%s"' % (what.split(':', 1)[1], k))
            # вставляем перед закрывающей скобкой тега
            if raw.endswith('/>'):
                raw = raw[:-2].rstrip() + ' ' + ' '.join(extra) + '/>'
            else:
                raw = raw[:-1].rstrip() + ' ' + ' '.join(extra) + '>'
        parts.append(raw)

    for child in node.children:
        parts.append(rebuild(child, marks))
    return ''.join(parts)


def process(path, strings, apply_changes):
    src = io.open(path, encoding='utf-8').read()

    builder = Builder()
    builder.feed(src)
    builder.close()

    rebuilt = serialize(builder.root)
    if rebuilt != src:
        return False, src, rebuilt, 0

    marks = {}
    collect(builder.root, strings, marks)

    if apply_changes:
        out = rebuild(builder.root, marks)
        io.open(path, 'w', encoding='utf-8', newline='\n').write(out)
    return True, src, rebuilt, len(marks)


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else 'check'
    strings = {}
    ok = True

    for name in PAGES:
        if not os.path.exists(name):
            print('  пропуск (нет файла):', name)
            continue
        good, src, rebuilt, n = process(name, strings, mode == 'apply')
        if not good:
            ok = False
            print('  СБОЙ сборки:', name)
            # покажем первое расхождение
            for i in range(min(len(src), len(rebuilt))):
                if src[i] != rebuilt[i]:
                    print('    расхождение на позиции', i)
                    print('    было:  ', repr(src[i - 60:i + 60]))
                    print('    стало: ', repr(rebuilt[i - 60:i + 60]))
                    break
        else:
            print('  %-20s размечено элементов: %d' % (name, n))

    if not ok:
        raise SystemExit('сборка не совпала, изменения не применялись')

    print('\n  уникальных строк:', len(strings))
    words = sum(len(re.sub(r'<[^>]+>', ' ', v).split()) for v in strings.values())
    print('  слов на перевод: ~%d' % words)

    if mode == 'apply':
        os.makedirs(os.path.join('assets', 'i18n'), exist_ok=True)
        out = os.path.join('assets', 'i18n', 'ru.json')
        io.open(out, 'w', encoding='utf-8', newline='\n').write(
            json.dumps(strings, ensure_ascii=False, indent=2, sort_keys=True))
        print('  словарь:', out)


if __name__ == '__main__':
    main()
