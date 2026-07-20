"""Sphinx extension to inject topic-category prev/next nav under headings.

The "In this documentation" table on the home page groups pages into topic
categories that cut across the Diataxis pillars, so the toctree prev/next
cannot express their order. This extension reads that table and injects a nav
strip under each heading the table points at, keeping the table the single
source of the ordering.

Entries are matched per section rather than per page: a `:ref:` label resolves
to a section anchor, and several pages carry more than one table entry
(reference/cli/workshop holds five, spanning two categories).
"""

from docutils import nodes
from sphinx import addnodes
from sphinx.errors import NoUri
from sphinx.util import logging

logger = logging.getLogger(__name__)

# Docname of the page holding the category table, and the auto-generated
# anchor of its "In this documentation" section, which the category name in
# each strip links back to.
INDEX_DOCNAME = 'index'
INDEX_ANCHOR = 'in-this-documentation'


def _find_category_table(doctree):
    """Return the "In this documentation" table node, or None.

    Identified by the `:class: borderless` set on the list-table directive,
    which docutils preserves on the table node.
    """
    for table in doctree.findall(nodes.table):
        if 'borderless' in table.get('classes', ()):
            return table
    return None


def _extract_rows(table, labels):
    """Yield (category, items) per table row, items being ordered links.

    Each item is a (text, docname, node_id) triple, where text is the link
    text used in the table rather than the target's own title.

    Args:
        table: The category table node, from an unresolved doctree.
        labels: The std domain's label map.
    """
    # Only tbody, so that adding :header-rows: later cannot turn a header into
    # a category.
    for tbody in table.findall(nodes.tbody):
        for row in tbody.findall(nodes.row):
            entries = [child for child in row.children if isinstance(child, nodes.entry)]
            if len(entries) < 2:
                continue

            category = entries[0].astext().strip()
            items = []

            for xref in entries[1].findall(addnodes.pending_xref):
                if xref.get('refdomain') != 'std' or xref.get('reftype') != 'ref':
                    continue
                target = labels.get(xref['reftarget'])
                if target is None:
                    # The std domain already warns about undefined labels.
                    continue
                docname, node_id, _ = target
                items.append((xref.astext(), docname, node_id))

            if category and items:
                yield category, items


def _build_index(app, env):
    """Return (by_docname, signature) extracted from the category table.

    by_docname maps a docname to the list of nav entries to inject into it.
    The signature covers every resolved target, so it changes when a target
    page moves even though the table itself did not.
    """
    table = _find_category_table(env.get_doctree(INDEX_DOCNAME))
    if table is None:
        logger.info('category_nav: no category table found in %s', INDEX_DOCNAME)
        return {}, ()

    by_docname = {}
    signature = []

    for category, items in _extract_rows(table, env.domains['std'].labels):
        for position, (text, docname, node_id) in enumerate(items):
            by_docname.setdefault(docname, []).append({
                'category': category,
                'node_id': node_id,
                'prev': items[position - 1] if position > 0 else None,
                'next': items[position + 1] if position < len(items) - 1 else None,
            })
            signature.append((category, position, text, docname, node_id))

    return by_docname, tuple(signature)


def collect_category_nav(app, env):
    """Extract the category table, and force affected pages to rewrite.

    Sphinx cannot see that every target page depends on the table, so editing
    the table would otherwise leave their nav strips stale. Docnames returned
    from `env-updated` are added to the set of pages to write, which is all a
    `doctree-resolved` injection needs; noting a dependency would instead
    force a needless re-read of every target.
    """
    if app.builder.format != 'html':
        return None

    by_docname, signature = _build_index(app, env)

    previous_signature = getattr(env, 'category_nav_signature', None)
    previous_docnames = getattr(env, 'category_nav_docnames', set())

    env.category_nav_by_docname = by_docname
    env.category_nav_signature = signature
    env.category_nav_docnames = set(by_docname)

    # `env-updated` fires on every build; rewriting only on a change keeps
    # autobuild from touching every target page on each save.
    if previous_signature is None or previous_signature == signature:
        return None

    # Pages dropped from the table are rewritten too, to shed their strips.
    affected = sorted((previous_docnames | set(by_docname)) & env.found_docs)
    logger.info('category_nav: table changed, rewriting %d page(s)', len(affected))
    return affected


def _nav_node(app, fromdocname, entry):
    """Build the nav strip: category name, then prev/next links."""

    def link(text, docname, node_id, classes):
        uri = app.builder.get_relative_uri(fromdocname, docname) + '#' + node_id
        reference = nodes.reference('', '', internal=True, refuri=uri, classes=classes)
        reference += nodes.Text(text)
        return reference

    nav = nodes.paragraph('', '', classes=['category-nav'])
    nav += link(entry['category'], INDEX_DOCNAME, INDEX_ANCHOR, ['category-nav-category'])

    siblings = nodes.inline('', '', classes=['category-nav-siblings'])
    if entry['prev']:
        text, docname, node_id = entry['prev']
        siblings += link('← ' + text, docname, node_id, ['category-nav-prev'])
    if entry['prev'] and entry['next']:
        siblings += nodes.inline('', '|', classes=['category-nav-sep'])
    if entry['next']:
        text, docname, node_id = entry['next']
        siblings += link(text + ' →', docname, node_id, ['category-nav-next'])
    nav += siblings

    return nav


def inject_category_nav(app, doctree, fromdocname):
    """Insert a nav strip under each heading this page contributes to a category."""
    if app.builder.format != 'html':
        return

    entries = getattr(app.env, 'category_nav_by_docname', {}).get(fromdocname)
    if not entries:
        return

    for entry in entries:
        section = doctree.ids.get(entry['node_id'])
        if not isinstance(section, nodes.section) or not isinstance(section[0], nodes.title):
            continue
        if any('category-nav' in child.get('classes', ()) for child in section.children):
            continue

        try:
            nav = _nav_node(app, fromdocname, entry)
        except NoUri:
            continue

        section.insert(1, nav)


def setup(app):
    """
    Register the category-nav extension with Sphinx.

    Returns:
        dict: Extension metadata with version and parallel_read_safe/parallel_write_safe flags.
    """
    app.connect('env-updated', collect_category_nav)
    app.connect('doctree-resolved', inject_category_nav)

    return {
        'version': '0.1',
        'parallel_read_safe': True,
        'parallel_write_safe': True,
    }
