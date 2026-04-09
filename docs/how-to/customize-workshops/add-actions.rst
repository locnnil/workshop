.. _how_add_actions:

.. meta::
   :description: How-to guide on adding actions to workshops, enabling automation and
                 enhanced functionality without modifying SDKs.

How to add actions to your workshop
===================================

.. @tests made redundant by tests/main/exec/task.yaml

This guide explains how to add actions to an existing workshop
to automate mundane tasks and enhance its functionality
without modifying the SDKs themselves
or running lengthy :command:`workshop exec` commands.


Adding actions
--------------

To add actions,
edit your :file:`workshop.yaml` file,
adding named action definitions in :program:`bash` format under :samp:`actions`
and making use of the features provided by the SDKs in your workshop

Here's an example of a workshop definition with two actions
that use the capabilities provided by the sketch SDK
from the :ref:`tut_sketch_sdks` tutorial section:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 7-11

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: go
       channel: 1.26
   
   actions:
     lint: |
       golangci-lint run --out-format=colored-line-number -c .golangci.yaml
     shellcheck: |
       git ls-files | file --mime-type -Nnf- | grep shellscript | cut -f1 -d: | xargs shellcheck --check-sourced --external-sources


Unlike changes in SDK layout or base,
action updates do not require a :command:`workshop refresh`.


Running actions
---------------

To execute an action,
use the :command:`workshop run` command.
Specify the workshop and its action,
with an optional separator (:samp:`--`):

.. @artefact workshop run

.. code-block:: console

   $ workshop run dev -- lint

     main.go:1:
     ./main.go:5:2: "os" imported and not used (typecheck)
     package main

   $ workshop run dev shellcheck
   
     In 1.sh line 10:
     cat /etc/passwd | grep root
         ^---------^ SC2002 (style): Useless cat. Consider 'cmd < file | ..' or 'cmd file | ..' instead.


When you run an action using :command:`workshop run`,
any additional arguments provided after the action name
are passed directly to the action itself:

.. code-block:: console

   $ workshop run dev -- lint --verbose


In projects with a single workshop, the workshop name is optional:

.. code-block:: console

   $ workshop run -- lint


Conclusion
----------

By adding actions to your workshop,
you can streamline your daily |ws_markup| workflows
and reduce the risk of typing errors.

For more advanced scripting capabilities,
consider exploring additional features of the SDKs,
such as hooks.


See also
--------

Explanation:

- :ref:`exp_workshop_definition_actions`
- :ref:`exp_sdk_hooks`


Reference:

- :ref:`ref_workshop_exec`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_run`
- :ref:`ref_workshop_actions`
