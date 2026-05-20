:hide-toc:

.. _exp_workshop_cli:

.. meta::
   :description: Documentation of the workshop CLI, detailing its role in
                 managing the lifecycle of workshops and interacting with the
                 workshopd daemon via a REST API.

workshop (CLI)
==============

.. @artefact workshop (CLI)
.. @artefact workshopd

|ws_markup| includes an eponymous command-line utility,
:command:`workshop`;
it is the daily go-to instrument for regular users,
with a set of commands that govern the entire lifecycle of a
:ref:`workshop <exp_workshop>`.

There are several categories of commands that vary by their purpose:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 10 11 20

   * - Actions
     - Commands
     - What they do

   * - Create, update, delete
     - :command:`launch`,
       :command:`refresh`,
       :command:`remove`,
       :command:`restore`,
       :command:`start`,
       :command:`stop`
     - Control a workshop's existence and runtime state,
       from first launch to refresh, restore, and removal.

   * - Customize
     - :command:`sketch-sdk`,
       :command:`sketches`
     - Augment a workshop with project-specific customizations
       through sketch SDKs.

   * - Enumerate
     - :command:`info`,
       :command:`list`
     - List the workshops in a project and inspect their current details.

   * - Track changes
     - :command:`changes`,
       :command:`tasks`
     - Review recent changes to the workshops in a project
       and the tasks that make up each change.

   * - Manage connections
     - :command:`connect`,
       :command:`connections`,
       :command:`disconnect`,
       :command:`remount`
     - Wire interface plugs and slots between SDKs,
       list existing connections, and remount their sources.

   * - Run shell commands
     - :command:`exec`,
       :command:`shell`
     - Run an ad-hoc command in a workshop
       or open an interactive shell inside it.

   * - Run named actions
     - :command:`actions`,
       :command:`run`
     - List and invoke the named actions
       defined in a workshop's :samp:`actions:` section.

   * - Manage warnings
     - :command:`okay`,
       :command:`warnings`
     - List warnings raised by the daemon and acknowledge them.


For an end-to-end example of putting these commands to use,
refer to the :ref:`tutorial <tut_index>`.

.. note::

   The utility talks to the |ws_markup| daemon,
   :program:`workshopd`, via a REST API,
   so alternatives are possible and, in fact, encouraged.


See also
--------

Reference:

- :ref:`ref_cli`


Tutorial:

- :ref:`tut_get_started`
