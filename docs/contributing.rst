How to contribute
=================

We believe everyone has something valuable to contribute,
whether you're a coder, a writer or a tester.
Here's how and why of your potential involvement:

- **Why join us?** Collaborate with fellow-minded colleagues, grow your skills,
  connect with diverse professionals, and make a tangible difference.

- **What you get**: Personal growth, recognition for your contributions, early
  access to new features and the satisfaction of seeing your work used and
  appreciated.

- **Start early, start simple**: Dive into code contributions, help improve
  documentation, or be among the first to test new features. Your participation
  matters, regardless of your experience level or the scope of your
  contribution.

Please follow the guidelines below for effective and meaningful contributions.

Coding
------

Commit messages
~~~~~~~~~~~~~~~

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


However, documentation-related commits use the ``Doc:`` type prefix
with an optional scope in square brackets:

.. code-block:: none

   Doc[chore]: Align references


PR descriptions should follow the PR template checklist
that largely reiterates this section.


Reversibility
~~~~~~~~~~~~~

When making decisions that might be costly to reverse,
explicitly state the rationale in the PR description.
This helps to understand the reasoning and collaborate better.


Coding standards
~~~~~~~~~~~~~~~~

- **Avoid nested conditions:**
  Refrain from nesting conditions to enhance readability and maintainability.

- **Eliminate dead code and redundant comments:**
  Remove unused or obsolete code and comments.
  This promotes a cleaner codebase and reduces confusion.

- **Normalize symmetries:**
  Handle identical operations consistently, using a uniform approach.
  This also improves consistency and readability.


Error handling
~~~~~~~~~~~~~~

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


Code structure
~~~~~~~~~~~~~~

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


Testing
-------

Make sure to run unit and integration tests before submitting a PR.
Workshop uses a ``go test``-compatible
`gocheck <https://pkg.go.dev/gopkg.in/check.v1#section-readme>`_:

.. code-block:: console

   $ go test ./...
   $ go test -check.f <TestName|SuiteName>


To run end-to-end tests and integration tests with
`Spread <https://github.com/snapcore/spread>`_:

.. code-block:: console

   $ go install github.com/snapcore/spread/cmd/spread@latest
   $ spread


Documentation
-------------

All documentation resides in the ``docs/`` directory.
To build and run it at ``127.0.0.1:8000``:

.. code-block:: console

   $ make run


To suggest changes online, use the GitHub link in the footer of the page
or submit a PR, limiting it to the ``docs/`` directory
and following our internal
`Sphinx and Read the Docs guide
<https://canonical-documentation-with-sphinx-and-readthedocscom.readthedocs-hosted.com/>`_.