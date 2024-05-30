.. _exp_workshop:

Workshops
=========

A *workshop*
(lowercase; not to be confused with |project_markup| itself)
is a container that is described in a definition file,
which is associated with a :ref:`project directory <exp_projects>`.
Currently, these containers are hosted by
`LXD <https://documentation.ubuntu.com/lxd/en/latest/>`__,
but it's not recommended to rely on this implementation detail.


.. _exp_workshop_def:

Workshop definition
-------------------

This is a file named :file:`.workshop.<NAME>.yaml`
that lists the base image of the workshop
and the specific components installed on top of it.
The definition acts as a single source of truth about the workshop.
It usually takes a few tries
to produce a definition that works for your project,
so you can edit and update the file iteratively.

A simple definition might look like this:

.. code-block:: yaml
   :caption: .workshop.golang.yaml

   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable


It specifies a *base* and an *SDK*.
A more complete definition would usually list several SDKs
that use different :ref:`interfaces <exp_interfaces>`,
packages and :ref:`hooks <exp_sdk_hooks>`.


.. _exp_base:

Base image
----------

The base is a supported OS image
that is used as the basis for the workshop.
Currently, it can be either
:samp:`ubuntu@20.04`, :samp:`ubuntu@22.04` or :samp:`ubuntu@24.04`.


See also
--------

Explanation:

- :ref:`exp_projects`
- :ref:`exp_sdk`