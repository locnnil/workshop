Workshop
========

**Workshop is a tool that automates intricate prerequisite setup
for your projects**.

**Define your dev environment in straightforward YAML**.
The tool consumes the definition to create a contained workshop,
installs the dependencies it lists as a number of SDKs,
and attaches their life cycle hooks for run-time control.
IDEs such as Visual Studio Code or JupyterLab
can discover workshops and use them in their operation,
tidying up your system and streamlining your work.

**Untangle the know-how that was weaved into your project**.
An environment that could take hours of setup
can be launched with one command;
workshops enhance issue reproduction across platforms,
facilitate collaboration in code reviews,
and confine hackish experiments in lightweight containers.

**Mitigate your setup's complexity with Workshop.**
AI/ML, robotics, IoT, EdTech, and similar domains
commonly have less-than-trivial project layouts
that depend on multiple Linux distributions,
a plethora of SDKs from different publishers,
and a grocery list of libraries and programming languages.


Getting Started
---------------

Follow the sections below
or refer to the
`Tutorial
<https://canonical-workshop.readthedocs-hosted.com/en/latest/tutorial/>`_
in our docs for a more detailed introduction to Workshop.


------------
Installation
------------

Workshop requires
`LXD <https://documentation.ubuntu.com/lxd/en/latest/>`_
for low-level operation:

.. code-block:: bash

   sudo snap install lxd
   sudo lxd init --auto

Build and install the ``workshop`` snap:

.. code-block:: bash

   git clone git@github.com:canonical/workshop.git
   # -- or --
   git clone https://github.com/canonical/workshop.git

   cd workshop
   sudo snap install snapcraft --classic
   snapcraft
   sudo snap install --devmode ./workshop_0.1.0_amd64.snap

Launching workshops
--------------------

In the root directory of the project
that you want to use with Workshop,
create a workshop definition file named ``.workshop.<NAME>.yaml``
to list your project's prerequisites,
then run ``workshop launch <NAME>``:

.. code-block:: bash

   cat > .workshop.nimble.yaml <<EOF -
   name: nimble
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable
     openjdk:
       channel: latest/stable
   EOF

   workshop launch nimble


Workshop downloads and installs the SDKs your definition lists;
the project is now ready to use them.


Testing
-------

Workshop uses a ``go test``-compatible
`gocheck <https://pkg.go.dev/gopkg.in/check.v1#section-readme>`_:

.. code-block:: bash

   go test ./...
   go test -check.f <TestName|SuiteName>


To run end-to-end tests and integration tests with
`Spread <https://github.com/snapcore/spread>`_:

.. code-block:: bash

   go install github.com/snapcore/spread/cmd/spread@latest
   spread
