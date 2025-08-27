.. _how_vscode_run_in_browser:

.. meta::
   :description: How-to guide on running a VS Code Server instance in a workshop
                 and accessing it via a browser.

How to run VS Code in your browser
==================================

.. @artefact workshop (container)

This guide explains how to use Visual Studio Code with Workshop 
by running a VS Code Server instance inside your workshop
and accessing it through your browser.

To do that, add the :samp:`code-server` SDK
and configure a tunnel interface plug for the :samp:`system` SDK:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 5-10

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: system
       plugs:
         code-server:
           interface: tunnel
           endpoint: 8090
     - name: code-server
       channel: 24.04/stable


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
