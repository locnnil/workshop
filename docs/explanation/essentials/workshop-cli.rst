:hide-toc:

.. _exp_workshop_cli:

workshop (CLI)
==============

|project_markup| includes an eponymous command-line utility,
:program:`workshop`;
it is the daily go-to instrument for regular users,
with a set of commands that govern the entire life cycle of a
:ref:`workshop <exp_workshop>`.

.. note::

   The utility talks to the |project_markup| daemon,
   :program:`workshopd`, via a REST API,
   so alternatives are possible and, in fact, encouraged.

There are several categories of commands that vary by their purpose:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 1 1 2

   * - Actions
     - Commands
     - What they do

   * - Create, update, delete
     - :ref:`launch <ref_workshop_launch>`,
       :ref:`refresh <ref_workshop_refresh>`,
       :ref:`remove <ref_workshop_remove>`
     - Control a workshop's existence;
       not to be confused with starting or stopping a workshop.

   * - Start, stop
     - :ref:`start <ref_workshop_start>`,
       :ref:`stop <ref_workshop_stop>`
     - Begin and end the run-time life cycle of an existing workshop.

   * - Explore, troubleshoot
     - :ref:`list <ref_workshop_list>`,
       :ref:`info <ref_workshop_info>`,
       :ref:`changes <ref_workshop_changes>`,
       :ref:`tasks <ref_workshop_tasks>`
     - Enumerate workshops, list their details and recent activities.

   * - Utilise
     - :ref:`exec <ref_workshop_exec>`
     - Run commands inside a workshop.

For an end-to-end example of putting these commands to use,
refer to the :ref:`tutorial <tutorial>`.
