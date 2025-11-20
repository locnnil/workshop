// Make toctree entries with :class: flat-toctree non-clickable containers in sidebar
// This script uses pre-computed absolute docnames from the Sphinx build
// Also makes the immediate parent of the current page non-clickable if it has children
document.addEventListener('DOMContentLoaded', function() {
    // Check if flat-toctree docnames were injected by the Sphinx extension
    if (!window.FLAT_TOCTREE_PATHS || !Array.isArray(window.FLAT_TOCTREE_PATHS)) {
        return;
    }

    // Convert docnames to a Set for O(1) lookup
    var docnameSet = new Set(window.FLAT_TOCTREE_PATHS);

    // Get current page's docname from the URL path
    var currentPath = window.location.pathname;
    // Extract the path after the build directory, handling /path/, /path/index.html, etc.
    // Remove trailing /index.html or just trailing /
    currentPath = currentPath.replace(/\/index\.html$/, '').replace(/\/$/, '');
    // Extract the path components (e.g., /how-to/ becomes 'how-to')
    var pathParts = currentPath.split('/').filter(function(p) { return p.length > 0; });
    var currentDocname = pathParts.join('/');

    // Helper to resolve a relative href to an absolute docname based on current page
    function resolveHrefToDocname(href, currentDoc) {
        // Remove leading './'
        href = href.replace(/^\.\//, '');

        // Remove trailing slash
        if (href.endsWith('/')) {
            href = href.slice(0, -1);
        }

        // Remove trailing '/index.html' or '/index'
        href = href.replace(/\/index(\.html)?$/, '');

        // If it starts with '../', resolve relative to current document
        if (href.startsWith('../')) {
            var currentParts = currentDoc.split('/');
            var hrefParts = href.split('/');

            for (var i = 0; i < hrefParts.length; i++) {
                if (hrefParts[i] === '..') {
                    if (currentParts.length > 0) {
                        currentParts.pop();
                    }
                } else if (hrefParts[i] === '.') {
                    continue;
                } else if (hrefParts[i]) {
                    currentParts.push(hrefParts[i]);
                }
            }

            return currentParts.join('/');
        }

        // If it's a simple relative path (no ../ or /), it's relative to current page's directory
        // For a page like 'how-to', the href 'customize-workshops/' becomes 'how-to/customize-workshops'
        if (!href.startsWith('/') && href.indexOf('../') === -1) {
            // Current page IS a directory (e.g., 'how-to' means we're in the how-to directory)
            if (currentDoc) {
                return currentDoc + '/' + href;
            }
            return href;
        }

        // Otherwise return as-is (absolute path)
        return href;
    }

    // Find the current page link in the sidebar
    var currentPageLi = document.querySelector('.sidebar-tree li.current-page');

    // Also make the immediate parent of the current page non-clickable if it has children
    if (currentPageLi) {
        var parentUl = currentPageLi.parentElement;
        if (parentUl && parentUl.tagName === 'UL') {
            var parentLi = parentUl.parentElement;
            if (parentLi && parentLi.tagName === 'LI' && parentLi.classList.contains('has-children')) {
                parentLi.classList.add('flat-toctree');
            }
        }
    }

    // Only check sidebar links (more efficient than all links)
    var sidebarLinks = document.querySelectorAll('.sidebar-tree a[href]');

    sidebarLinks.forEach(function(link) {
        var href = link.getAttribute('href');
        if (!href) return;

        // Convert href to absolute docname
        var docname = resolveHrefToDocname(href, currentDocname);

        if (docnameSet.has(docname)) {
            // This link should be non-clickable
            var li = link.closest('li.has-children');
            if (li) {
                li.classList.add('flat-toctree');
            }
        }
    });
});

