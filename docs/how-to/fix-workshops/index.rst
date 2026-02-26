.. _how_fix_workshops:

.. meta::
   :description: How-to guides on fixing workshops, including debugging
                 issues, troubleshooting installations, and purging workshops.

How to fix workshops
====================

These how-to guides help you troubleshoot
and resolve issues with your workshops.


Diagnose and resolve issues
---------------------------

When a workshop misbehaves, start by tracing the root cause.
If the problem involves resource conflicts between SDKs,
the plug conflict guide walks through that specific scenario:

.. toctree::
   :maxdepth: 1

   Debug issues in workshops <debug-issues>
   Resolve plug conflicts <resolve-plug-conflicts>


Repair or reset
---------------

If debugging doesn't help, the issue may lie
in the |ws_markup| installation itself,
or the workshop may need to be purged and recreated from scratch:

.. toctree::
   :maxdepth: 1

   Fix the installation <fix-installation>
   Purge workshops <purge>
