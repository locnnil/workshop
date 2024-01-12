.. _exp_content_interface:

Content interface
=================

Introduction
------------

The content
:ref:`interface <exp_interfaces_plugs_slots>`
exposes host file system locations
to individual SDKs
by mounting them inside the :ref:`workshop <exp_workshop>`
that references these SDKs.


Content interface plug
----------------------

An essential element here is the content interface plug
that is declared in the :ref:`SDK definition <exp_sdk_def>`.
A basic structure includes the name of the plug itself,
the interface (:samp:`content`)
and the intended target path inside the workshop:

.. code-block:: yaml
   :caption: sdk.yaml

   name: go
   title: Go SDK
   base: ubuntu@20.04
   summary: The Go programming language
   description: |
     Go is an open source programming language that enables the production
     of simple, efficient and reliable software at scale.

   plugs:
     mod-cache:
       interface: content
       target: /home/workshop/go/pkg/mod


This definition creates a plug called :samp:`mod-cache`
that does the following:

- Sets its :samp:`interface` type to :samp:`content`,
  which makes it a content interface plug

- Points the :samp:`target` directory *inside the workshop*
  to :file:`/home/workshop/go/pkg/mod/`;
  it will be mounted to a file directory on the host system
  that |project_markup| designates at run-time

Overall, the intent of this declaration is
to use a directory
(which |project_markup| automatically allocates for the slot)
for persisting the
`module cache <https://go.dev/ref/mod#module-cache>`__
in the host file system
when the workshop stops.


Content interface slot
----------------------

To let SDKs access the host file system,
|project_markup| creates a slot per each content interface plug.

.. note::

   Currently, content can only be exposed by |project_markup| itself,
   but can't be shared between two workshops directly.


At run-time, the plug is connected to the slot;
after that, it's time for some
:ref:`validation and policy checks <exp_interfaces_validation>`
that |project_markup| does internally.
This involves making sure that the plug declaration is correct;
the plug is allowed to be installed and auto-connected;
and the destination directory actually exists.

If the content interface plug passes the checks,
the :samp:`target` directory, as defined by the plug,
is mounted on the host file system.
That's where the module cache from our example will end up;
the best part is that it will be preserved between workshop operations such as
:ref:`refresh <ref_workshop_refresh>`,
:ref:`start <ref_workshop_start>` and :ref:`stop <ref_workshop_stop>`,
so you can benefit from a pre-populated module cache without doing extra work.
