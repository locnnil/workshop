.. _how_vscode_connect_remote:

.. meta::
   :description: How-to guide on connecting a local Visual Studio Code
                 instance to a remote workshop.

How to connect your local VS Code to a workshop
===============================================

.. @artefact workshop (container)

This article shows how to use Visual Studio Code with Workshop
by connecting your local VS Code to a remote workshop environment.

First, you'll need to have the `remote development extension pack
<https://code.visualstudio.com/docs/remote/remote-overview>`__
installed.

After that, add the :samp:`vscode-remote` SDK to your workshop definition:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 4-5

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: vscode-remote
       channel: noble/stable


Launch the workshop.
Next, the output from :command:`workshop tasks` will hint at the next steps:

.. code-block:: console

   $ workshop tasks

     ...
     VS Code → Open Remote Window → Connect to host → workshop@10.41.49.51


Follow this guidance and type in the SSH address listed in the output
(:samp:`workshop@10.41.49.51` in the sample above).
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

- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_tasks`
