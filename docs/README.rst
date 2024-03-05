Workshop
========

.. image:: https://readthedocs.com/projects/canonical-workshop/badge/?version=latest&token=a8c81a46da98f75a366a1eef905457dadfa50c23cf3a1c1929a81af05ffea85d
   :target: https://canonical-workshop.readthedocs-hosted.com/en/latest/?badge=latest
   :alt: Documentation Status

**A tool for defining and managing complex development environments**.

**Define your prerequisites and dependencies in simple YAML**.
Workshop consumes the definition to create a contained workshop,
installs the components as a set of SDKs
and attaches custom actions for run-time control.
IDEs such as Visual Studio Code or JupyterLab can discover workshops
and use them in day-to-day operations,
tidying up your system and streamlining your work.

**Focus on your project, not your setup**.
An environment that could take hours to configure
can now be launched with a single command.
Workshop improves cross-platform issue reproduction,
preserves context in discussions or reviews
and confines bold experiments to transparent sandboxes.

**For those who build and maintain complex, error-prone workspaces**.
AI/ML, robotics, IoT, EdTech and similar domains
typically use less-than-trivial project layouts
that depend on multiple Linux distributions or images,
a plethora of SDKs from many vendors
and a grocery list of libraries and languages.
That’s where Workshop thrives.


Getting Started
---------------

Follow the sections below
or refer to the
`Tutorial
<https://canonical-workshop.readthedocs-hosted.com/en/latest/tutorial/>`_
in our docs for a more detailed introduction to Workshop.

To know more about `SDKcraft <https://github.com/canonical/sdkcraft>`_,
the user-facing counterpart to Workshop,
start with the `Tutorial
<https://canonical-sdkcraft.readthedocs-hosted.com/en/latest/tutorial/>`_.


------------
Installation
------------

Workshop requires
`LXD <https://canonical.com/lxd>`_
for low-level operation:

.. code-block:: console

   sudo snap install lxd
   sudo lxd init --auto


Build and install the ``workshop`` snap, for example:

.. code-block:: console

   git clone git@github.com:canonical/workshop.git  # or git clone https://github.com/canonical/workshop.git

   cd workshop
   sudo snap install snapcraft --classic
   snapcraft clean && snapcraft
   sudo snap install --dangerous --classic ./workshop_0.1.0_amd64.snap


Launching workshops
-------------------

In the root directory of the project
that you want to use with Workshop,
create a workshop definition file named ``.workshop.<NAME>.yaml``
to list your project's prerequisites,
then run ``workshop launch <NAME>``:

.. code-block:: console

   cat > .workshop.golang.yaml <<EOF -
   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable
   EOF

   workshop launch golang


Workshop downloads and installs the SDKs your definition lists;
the project is now ready to use them.


Testing
-------

Workshop uses a ``go test``-compatible
`gocheck <https://pkg.go.dev/gopkg.in/check.v1#section-readme>`_:

.. code-block:: console

   go test ./...
   go test -check.f <TestName|SuiteName>


To run end-to-end tests and integration tests with
`Spread <https://github.com/snapcore/spread>`_:

.. code-block:: console

   go install github.com/snapcore/spread/cmd/spread@latest
   spread


Contributing
------------

Please follow the guidelines below for effective and meaningful contributions.


---------------
Commit messages
---------------

Workshop uses a style that differs from conventional commits in capitalisation:
.. code-block:: none

   Ensure correct permissions and ownership for the content mounts
    
    * Work around an LXD issue regarding empty dirs:
      https://github.com/canonical/lxd/issues/12648
    
    * Ensure the source directory is owned by the user running a workshop.

   Links:
   - ...
   - ...

The messages rarely if ever state the type of the commit,
e.g. ``fix``, ``feat``, etc.;
these are used for branch naming, for example:

- ``canonical/feat/workspace-start``
- ``canonical/fix/spread-tests-github``
- ``canonical/chore/update-lxd``


However, docs-related commits use the ``Doc:`` type prefix
with an optional scope in square brackets:

.. code-block:: none

   Doc[chore]: Align references


PR descriptions should follow the PR template checklist
that largely reiterates this section.


-------------
Reversibility
-------------

When making decisions that might be costly to reverse,
explicitly state the rationale in the PR description.
This helps to understand the reasoning and collaborate better.


----------------
Coding standards
----------------

- **Avoid nested conditions:**
  Refrain from nesting conditions to enhance readability and maintainability.

- **Eliminate dead code and redundant comments:**
  Remove unused or obsolete code and comments.
  This promotes a cleaner codebase and reduces confusion.

- **Normalize symmetries:**
  Handle identical operations consistently, using a uniform approach.
  This also improves consistency and readability.


--------------
Error handling
--------------

When handling errors or multiple returns,
follow a consistent pattern:

.. code-block:: go

   // one way to handle errors
   if err := f(); err != nil {
      ...
   }

   // one way to handle multiple returns
   val, err := f()
   if err != nil {
      ...
   }


--------------
Code structure
--------------

- **Check coupled code elements:**
  Verify that coupled code elements, files, and directories are adjacent.
  For instance, store test data close to the corresponding test code.

- **Group variable declaration and initialization:**
  Declare and initialize variables together
  to improve code organization and readability.

- **Divide large expressions:**
  Break down large expressions
  into smaller self-explanatory parts.
  Use multiple variables if necessary
  to make the code more understandable
  and choose names to reflect their purpose.

- **Use blank lines for logical separation:**
  Insert a blank line between two logically distinct sections of code.
  This improves its structure and makes it easier to comprehend.


-------
Testing
-------

Make sure to run unit and integration tests before submitting a PR.
