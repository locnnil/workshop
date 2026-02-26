.. _exp_workshop:

.. meta::
   :description: Workshop explanation documentation, providing access to
                 explanations of core workshop concepts, change tracking,
                 project management, and command-line interface usage.

Workshops
=========

.. @artefact workshop (container)

These articles explain the core idea behind |ws_markup|,
specifically the eponymous *workshop*.


Core concepts
-------------

A workshop is a container-based development environment
defined in YAML and hosted by LXD.
Projects are the directories that hold workshop definitions
and mount inside the running containers:

.. toctree::
   :maxdepth: 1

   concepts
   projects


Operations and tooling
----------------------

|ws_markup| tracks state through a system of changes and tasks,
and exposes its functionality through the :program:`workshop` CLI:

.. toctree::
   :maxdepth: 1

   changes-tasks
   workshop-cli
