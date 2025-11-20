.. _tut_work_with_interfaces:

.. meta::
   :description: Tutorial on using interfaces in workshops
                 to interact with the host system and other SDKs.

.. _tut_interfaces:

Work with interfaces
====================

This is the second section of the :ref:`four-part series <tut_index>`;
it explains how to work with interfaces.
It relies on the knowledge gained in the :ref:`tut_get_started` section,
where you learned how to create and run workshops.
Here, you will learn how to make better use of SDKs in your workshop
and integrate them with the host system.

.. @artefact interface
.. @artefact system SDK

SDKs use interfaces to interact in an organized manner,
exposing the resources they provide via slots and consuming them via plugs;
the layout of these plugs and slots is defined by the SDK publishers.

For uniformity, security, and control,
various host system capabilities (camera, GPU, and so on)
are also exposed to the workshop via the same interface mechanism
with a designated system SDK.


Manage connections
------------------

To check out the connected interfaces of a workshop, list the connections:

.. @artefact workshop connections

.. code-block:: console

   $ workshop connections

     Interface  Plug               Slot              Notes
     gpu        dev/ollama:gpu     dev/system:gpu    -
     mount      dev/ollama:models  dev/system:mount  -


This lists two interface plugs,
both provided by the :samp:`ollama` SDK under the :samp:`dev` workshop.

The first one is a GPU interface plug named :samp:`dev/ollama:gpu`.
It enables the workshop to use the host system's GPU
by connecting to the :samp:`dev/system:gpu` slot.

Also, there's a mount interface plug named :samp:`dev/ollama:models`.
As seen in the :command:`workshop info` output,
it was automatically connected at :ref:`launch <tut_launch>`
to the :samp:`dev/system:mount` slot,
indicated by the ellipsis in the :samp:`host-source` path.

Note that some interfaces are auto-connected, while some are not;
this depends on their built-in security policy defined by |ws_markup|.
For instance, you can't use the ssh-agent interface
without connecting it manually.

In any case, you can connect and disconnect interfaces at will.
To check the connection state, run :command:`workshop connections`:

.. @artefact workshop connect
.. @artefact workshop disconnect

.. code-block:: console

   $ workshop disconnect dev/ollama:models
   $ workshop connections

     Interface  Plug            Slot            Notes
     gpu        dev/ollama:gpu  dev/system:gpu  -

   $ workshop connect dev/ollama:models :mount
   $ workshop connections

     Interface  Plug               Slot              Notes
     gpu        dev/ollama:gpu     dev/system:gpu    -
     mount      dev/ollama:models  dev/system:mount  manual


You can remount a mount interface plug to a new location on the host.
For example, to preserve models,
conventionally stored under :file:`~/.ollama/models/`,
after the workshop is removed or use some models downloaded previously,
you can remount to a directory in your home:

.. @artefact workshop remount

.. code-block:: console
   :emphasize-lines: 16-19

   $ mkdir -p ~/.ollama/models
   $ workshop remount dev/ollama:models ~/.ollama/models
   $ workshop info

     name:     dev
     base:     ubuntu@22.04
     project:  /home/user/ollama-python-project
     status:   ready
     notes:    -
     sdks:
       system:
         installed:  (1)
       ollama:
         tracking:   22.04/edge
         installed:  0.9.6  2025-08-12  (690)
         mounts:
           models:
             host-source:      /home/user/.ollama/models
             workshop-target:  /home/workshop/.ollama/models


This makes :file:`/home/user/.ollama/models/` on the host
act as the models storage for the workshop.


Add plugs, slots
----------------

You can modify the behavior of the SDKs you installed in your workshop,
tailoring it to your needs and connecting them to other SDKs or the host system.

To do this, you add plugs and slots to the SDKs in the workshop definition,
allowing you to customize the initial plug and slot layout to your requirements.

This scenario usually arises
when you want to connect different SDKs running in the workshop
or expose some service from the workshop to the host system.

Let's look at an example.
Add the :samp:`jupyter` SDK to the workshop
to run Jupyter notebooks with the Ollama models:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 6,7

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: ollama
       channel: 22.04/edge
     - name: jupyter
       channel: 22.04/edge


.. code-block:: console

   $ workshop refresh

     "dev" refreshed


.. @artefact tunnel interface

Next, add the tunnel interface plug under the :samp:`system` SDK
in the workshop definition;
this exposes the Jupyter server, now available in the workshop,
to the host system at a port of your choice (here, :samp:`8989`):

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 8-12

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: ollama
       channel: 22.04/edge
     - name: jupyter
       channel: 22.04/edge
     - name: system
       plugs:
         jupyter:
           interface: tunnel
           endpoint: 127.0.0.1:8989


The slot we're going to connect this plug to comes from the SDK itself
and is named :samp:`dev/jupyter:jupyter`,
so you don't have to add it manually:

.. code-block:: console

   $ workshop connections --all

     Interface  Plug                Slot                      Notes
     gpu        dev/ollama:gpu      dev/system:gpu            -
     mount      dev/jupyter:venv    dev/system:mount          -
     mount      dev/ollama:models   dev/system:mount          -
     tunnel     -                   dev/ollama:ollama-server  -
     tunnel     dev/system:jupyter  dev/jupyter:jupyter       -


Refresh the workshop to enable the tunnel;
|ws_markup| will auto-connect the plug to the slot by matching their names
(the slot's name is :samp:`dev/jupyter:jupyter`).
Check the result using :command:`workshop info`:

.. code-block:: console

   $ workshop refresh

     "dev" refreshed

   $ workshop info

     ...
     sdks:
       system:
         installed:  (1)
         tunnels:
           jupyter:
             from:  127.0.0.1:8989/tcp
             to:    127.0.0.1:8888/tcp
     ...


Now, JupyterLab is available at the plug address:

.. code-block:: console

   $ curl http://127.0.0.1:8989/lab?

     <!doctype html><html lang="en">...


.. note::

   For additional details of using the tunnel interface, see this guide:
   :ref:`how_forward_ports`.


Next steps
----------

This was the last step in this tutorial section; you're halfway through!
Now you are familiar with the essentials of interfaces in |ws_markup|.

Your next step is to learn even more about workshop customization,
creating experimental SDKs quickly
with the :command:`workshop sketch-sdk` command;
proceed to the :ref:`tut_sketch_sdks` section.
