:hide-toc:

.. _exp_sdk_cli:

.. meta::
   :description: Documentation of the sdk CLI, detailing its role in
                 understanding available SDKs and which workshops use them.

sdk (CLI)
=========

.. @artefact sdk (CLI)

|ws_markup| includes an :command:`sdk` command-line utility;
it has a set of commands that make it easy to find and learn more about SDKs.

There are several commands that vary by their purpose:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 10 11 20

   * - Actions
     - Commands
     - What they do

   * - Discover
     - :command:`info`,
       :command:`list`
     - Enumerate SDKs, list their details and current usage.


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
