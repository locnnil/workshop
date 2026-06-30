.. _how_vscode_connect_remote:

.. meta::
   :description: How-to guide on connecting a local Visual Studio Code
                 instance to a remote workshop.

How to connect your local VS Code to a workshop
===============================================

.. @tests in tests/docs-how-to/connect-vscode/task.yaml

.. @artefact workshop (container)

A local VS Code instance can connect to a remote workshop environment
via the :samp:`vscode-remote` SDK,
giving you the full VS Code experience against |ws_markup|.

First, you'll need to have the `remote development extension pack
<https://code.visualstudio.com/docs/remote/remote-overview>`__
installed.

After that, add the :samp:`vscode-remote` SDK to your workshop definition:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 4

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: vscode-remote


Launch the workshop.
Then find the workshop hostname:

.. code-block:: console

   $ workshop info dev

   name:      dev
   base:      ubuntu@24.04
   project:   ~/my-project
   hostname:  dev.my-project.wp
   ...


In VS Code, choose :guilabel:`Open Remote Window`,
then :guilabel:`Connect to host`,
and type :samp:`workshop@dev.my-project.wp`
using the hostname from the :command:`workshop info` output.
In the terminal prompt, you'll see that the IDE is running inside your workshop.

.. note::

   If you're having trouble finding the :guilabel:`Connect to host` command,
   mind that it's enabled by the :samp:`Remote-SSH` extension
   from the extension pack mentioned above.


See also
--------

Explanation:

- :ref:`exp_workshop_definition`


Reference:

- :ref:`ref_workshop_info`
- :ref:`ref_workshop_launch`
