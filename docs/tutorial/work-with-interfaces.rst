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

.. @artefact interface
.. @artefact system SDK

SDKs use interfaces to interact in an organized manner,
exposing the resources they provide via slots and consuming them via plugs;
the layout of these plugs and slots is defined by the SDK publishers.

For uniformity, security and control,
various host system capabilities (camera, GPU, and so on)
are also exposed to the workshop via the same interface mechanism
with a designated system SDK.

To check out the connected interfaces of a workshop, list the connections:

.. @artefact workshop connections

.. code-block:: console

   $ workshop connections

     Interface  Plug              Slot              Notes
     mount      dev/go:mod-cache  dev/system:mount  -


This lists a mount interface plug named :samp:`dev/go:mod-cache`.
As seen in the :command:`workshop info` output,
it was automatically connected at :ref:`launch <tut_launch>`
to the :samp:`dev/system:mount` slot,
indicated by the ellipsis in the :samp:`host-source` path.

Some interfaces are auto-connected, while some are not;
this usually depends on their purpose.

In any case, you can connect and disconnect interfaces at will,
confirming the connection state with :command:`workshop connections`:

.. @artefact workshop connect
.. @artefact workshop disconnect

.. code-block:: console

   $ workshop disconnect dev/go:mod-cache
   $ workshop connections
   $ workshop connect dev/go:mod-cache :mount
   $ workshop connections


You can remount a mount interface plug to a new location on the host:

.. @artefact workshop remount

.. code-block:: console
   :emphasize-lines: 14

   $ workshop remount dev/go:mod-cache ~/mod/
   $ workshop info

     name:     dev
     base:     ubuntu@24.04
     project:  /home/user/hello-workshop
     status:   ready
     notes:    -
     sdks:
       go:
         tracking:   noble/stable
         installed:  1.23.3  2024-11-09  (54)
         mounts:
           mod-cache:
             host-source:      /home/user/mod
             workshop-target:  /home/workshop/go/pkg/mod


This makes :file:`/home/user/mod/` on the host
act as the Go modules cache for the workshop.

Lastly, you can add plugs and slots to the SDKs in the workshop definition,
allowing you to tailor the initial plug and slot layout to your requirements.
For instance, you could use the tunnel interface
with the system SDK to connect to a server running in the workshop.

.. @artefact tunnel interface

For a quick demo, let's install `Caddy <https://caddyserver.com/>`_
to serve files over HTTP:

.. code-block:: console

   $ workshop exec --env GOBIN=/project dev -- go install github.com/caddyserver/caddy/v2/cmd/caddy@latest
   $ cat <<EOF > Caddyfile
   :8080 {
           file_server
   }
   EOF
   $ echo 'Hello, Workshop!' > index.html


This builds Caddy inside the workshop,
installs it to the project directory,
configures it to run as a file server at port 8080
and creates an index file.

.. note::

   We added the index file to the project directory on the host;
   however, the server will be able to access it
   because the project directory is mounted inside the workshop.


To configure the tunnel interface,
add the following lines to the definition:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 6-14

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: go
       channel: noble/stable
       slots:
         caddy:
           interface: tunnel
           endpoint: localhost:8080
     - name: system
       plugs:
         caddy:
           interface: tunnel
           endpoint: localhost:8080


First, this defines a :samp:`go:caddy` slot under the :samp:`go` SDK,
used to expose the server running inside the workshop.
This slot isn't part of the SDK by default;
it's defined for this workshop only,
so other instances of the :samp:`go` SDK in other workshops won't have it.

Additionally, this adds a plug named :samp:`system:caddy`
to indicate that the system SDK in this workshop
can connect to a tunnel interface slot and expose it in the host system.

Refresh the workshop to enable the tunnel;
|ws_markup| matches the plug to the slot using their names,
then validates and enables the connection.
Check the result using :command:`workshop info`:

.. code-block:: console

   $ workshop refresh
   $ workshop info

     ...
     sdks:
       system:
         tunnels:
           server:
             from:  127.0.0.1:8080/tcp
             to:    127.0.0.1:8080/tcp
     ...

Then start the server at port 8080 (the slot):

.. code-block:: console

   $ workshop exec dev -- ./caddy start


By default,
:command:`exec` uses the :file:`/project/` directory in the workshop
as the current working directory
so Caddy will serve the files in it.
Finally, test the server on the host at port 8080 (the plug):

.. code-block:: console

   $ curl localhost:8080

     Hello, Workshop!


.. note::

   For additional details of using the tunnel interface, see this guide:
   :ref:`how_forward_ports`.

Next steps
----------

This was the last step in this tutorial section;
you are now familiar with the essentials of interfaces in |ws_markup|.

Your next step is to look at the different ways of building SDKs.

To continue learning about |ws_markup|,
proceed to the following tutorial sections:

- :ref:`tut_sketch_sdks`:
  Create experimental SDKs quickly
  using the :command:`workshop sketch-sdk` command.
  This tutorial section shows you
  how to run local SDK experiments without publishing them.

- :ref:`tut_craft_sdks`:
  Go through the complete process
  of building and publishing full-fledged SDKs to the SDK Store.
  This tutorial section covers the workflow for creating production-ready SDKs
  that can be shared with others.


Both sections build on what you've learned here
and expand your |ws_markup| skills.
