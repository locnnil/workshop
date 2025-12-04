"""Sphinx extension to flatten toctree entries."""

import json
import logging
import posixpath

from docutils import nodes
from sphinx import addnodes
from sphinx.environment.adapters.toctree import TocTree
from sphinx.errors import SphinxError

logger = logging.getLogger(__name__)


def _resolve_refuri_to_docname(refuri, source_docname):
    """Convert a refuri (relative path) to an absolute docname.

    Args:
        refuri: Relative path like 'architecture/' or '../develop-with-workshops/'
        source_docname: Source document like 'explanation/index' or 'how-to/index'

    Returns:
        Absolute docname like 'explanation/architecture' or 'how-to/develop-with-workshops'
    """
    if refuri.endswith('/'):
        refuri = refuri[:-1]

    base = posixpath.dirname(source_docname)
    return posixpath.normpath(posixpath.join(base, refuri))


def _find_flat_toctrees(doctree):
    """Yield (compound, toctree_node) pairs for compounds with flat-toctree class."""
    for compound in doctree.traverse(nodes.compound):
        if 'flat-toctree' not in compound.get('classes', ()):
            continue
        for toctree_node in compound.traverse(addnodes.toctree):
            yield compound, toctree_node


def _extract_parent_paths(resolved_toctree, source_docname):
    """Extract absolute docnames of parent items that have nested children.

    Args:
        resolved_toctree: The resolved toctree node
        source_docname: The document containing the toctree (e.g., 'explanation/index')

    Returns:
        List of absolute docnames (e.g., ['explanation/architecture', 'explanation/sdks'])
    """
    docnames = []

    bullet_list = next(iter(resolved_toctree.traverse(nodes.bullet_list)), None)
    if bullet_list:
        for list_item in bullet_list.children:
            if not isinstance(list_item, nodes.list_item):
                continue

            nested_list = next(
                (child for child in list_item.children if isinstance(child, nodes.bullet_list)),
                None,
            )

            if nested_list is not None:
                ref = next(iter(list_item.traverse(nodes.reference)), None)
                if ref:
                    refuri = ref.get('refuri')
                    if refuri:
                        docname = _resolve_refuri_to_docname(refuri, source_docname)
                        if docname:
                            docnames.append(docname)

    return docnames


def _flatten_bullet_list(bullet_list):
    """Remove parent items and promote their children to top level."""
    items_to_remove = []
    items_to_add = []

    for list_item in bullet_list.children:
        if not isinstance(list_item, nodes.list_item):
            continue

        nested_list = next(
            (child for child in list_item.children if isinstance(child, nodes.bullet_list)),
            None,
        )

        if nested_list is not None:
            for nested_item in nested_list.children:
                if isinstance(nested_item, nodes.list_item):
                    items_to_add.append(nested_item)

            items_to_remove.append(list_item)

    for item in items_to_remove:
        bullet_list.remove(item)

    for item in items_to_add:
        bullet_list.append(item)


def collect_flat_toctree_paths(app, env):
    """Pre-collect all flat-toctree paths before any pages are written."""
    if app.builder.format != 'html':
        return
    
    paths = getattr(env, 'flat_toctree_paths', None)
    if paths is None:
        env.flat_toctree_paths = set()
    else:
        paths.clear()

    toctree_adapter = TocTree(env)

    for docname in env.found_docs:
        doctree = env.get_doctree(docname)

        for compound, toctree_node in _find_flat_toctrees(doctree):
            try:
                resolved = toctree_adapter.resolve(docname, app.builder, toctree_node,
                                                   prune=True, maxdepth=0, titles_only=True)

                if resolved:
                    docnames = _extract_parent_paths(resolved, docname)
                    env.flat_toctree_paths.update(docnames)

            except (SphinxError, AttributeError, TypeError, KeyError):
                logger.exception("flat_toctree: Failed to resolve toctree in %s", docname)


def flatten_toctree(app, doctree, fromdocname):
    """Find toctrees with :class: flat-toctree and flatten them."""
    if app.builder.format != 'html':
        return
    
    toctree_adapter = TocTree(app.env)

    for compound, toctree_node in _find_flat_toctrees(doctree):
        try:
            resolved_toctree = toctree_adapter.resolve(fromdocname, app.builder, toctree_node,
                                                       prune=True, maxdepth=0, titles_only=True)
            if not resolved_toctree:
                continue

            bullet_list = next(iter(resolved_toctree.traverse(nodes.bullet_list)), None)
            if bullet_list:
                _flatten_bullet_list(bullet_list)
                toctree_node.replace_self(resolved_toctree)
        except (SphinxError, AttributeError, TypeError, KeyError):
            logger.exception("Failed to flatten toctree in %s", fromdocname)


def inject_flat_toctree_data(app, pagename, templatename, context, doctree):
    """Inject flat-toctree paths as JavaScript data into every page."""
    paths = getattr(app.env, 'flat_toctree_paths', None)
    if paths:
        data = json.dumps(sorted(paths))
        script = f'<script>window.FLAT_TOCTREE_PATHS = {data};</script>'
        context['body'] = context.get('body', '') + script


def setup(app):
    """
    Register the flat-toctree extension with Sphinx.

    Returns:
        dict: Extension metadata with version and parallel_read_safe/parallel_write_safe flags.
    """
    app.connect('env-updated', collect_flat_toctree_paths)
    app.connect('doctree-resolved', flatten_toctree)
    app.connect('html-page-context', inject_flat_toctree_data)

    return {
        'version': '0.1',
        'parallel_read_safe': True,
        'parallel_write_safe': True,
    }
