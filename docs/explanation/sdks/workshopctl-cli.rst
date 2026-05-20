:hide-toc:

.. _exp_workshopctl_cli:

.. meta::
   :description: Documentation of the workshopctl CLI, detailing its role as
                 the in-workshop helper invoked by SDK hooks to report back
                 to the workshopd daemon.

workshopctl (CLI)
=================

.. @artefact workshopctl
.. @artefact SDK hook

:command:`workshopctl` is a small in-workshop utility that
:ref:`SDK hooks <exp_sdk_hooks>` invoke to report SDK state
back to the |ws_markup| daemon over a restricted socket.
It runs inside a running workshop, not on the host,
and is not intended for end users to call directly.

There is one category of commands:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 10 11 20

   * - Actions
     - Commands
     - What they do

   * - Report SDK health
     - :command:`set-health`
     - Let the daemon know whether the SDK is :samp:`okay`,
       :samp:`waiting`, or in an :samp:`error` state,
       with an optional machine-readable error code
       and a human-readable message.


.. note::

   :command:`workshopctl` only works from an SDK hook context,
   where the daemon supplies a context cookie via the
   :envvar:`WORKSHOP_COOKIE` environment variable.
   Running it from an interactive shell returns
   ``cannot invoke workshopctl operation commands ... from outside of a workshop``.


See also
--------

Explanation:

- :ref:`exp_sdk_cli`
- :ref:`exp_workshop_cli`


Reference:

- :ref:`ref_cli`
- :ref:`ref_sdk_hooks`


Tutorial:

- :ref:`tut_get_started`
