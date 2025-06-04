.. _how_vscode_workshops:

How to use workshops with Visual Studio Code
============================================

.. @artefact workshop (container)

One of the goals for |ws_markup| is robust integration with developer tooling,
which obviously includes IDEs such as the ubiquitous VS Code.
There are two main options for using workshops with this IDE.


Connect your local VS Code to a workshop
----------------------------------------

The least intrusive way is to use the remote connectivity features of VS Code.

First, you'll need to have the `remote development extension pack
<https://code.visualstudio.com/docs/remote/remote-overview>`__
installed.

After that, add the :samp:`code-remote` SDK to your workshop definition:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 4-5

   name: vscode-remote
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

.. tip::

   If you're having trouble finding the :guilabel:`Connect to host` command,
   mind that it's enabled by the :samp:`Remote-SSH` extension
   from the extension pack mentioned above.


Run VS Code in your browser
---------------------------

Another, more portable option allows you to run a VS Code instance
in the workshop itself without the need to install it on the host,
accessing it via your browser.

To do that, add the :samp:`code-server` SDK
and configure a tunnel interface plug for the :samp:`system` SDK:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 5-10

   name: vscode-server
   base: ubuntu@24.04
   sdks:
     - name: system
       plugs:
         code-server:
           interface: tunnel
           endpoint: 8090
     - name: code-server
       channel: noble/stable


Launch the workshop.
After that, VS Code will be available in your browser at the plug address,
e.g. http://localhost:8090.
In the terminal prompt, you'll see that the IDE is running inside your workshop.


See also
--------

Explanation:

- :ref:`exp_system_sdk`
- :ref:`exp_tunnel_plug`
- :ref:`exp_workshop_definition`


Reference:

- :ref:`ref_tunnel_interface`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_tasks`
