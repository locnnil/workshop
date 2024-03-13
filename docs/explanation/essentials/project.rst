:hide-toc:

.. _exp_project:

Project
=======

Technically, a project is a directory
that contains one or many workshop definitions.

To initialise a directory as a project,
create a
:ref:`workshop definition <exp_workshop_def>`
in it
and run :command:`workshop launch`.
Launching a workshop from a project
establishes the relationship between these two,
which is required to actually start a workshop.

When the workshop is then started with :command:`workshop start`,
the project is mounted to it as :file:`/project/`,
and the :command:`workshop stop` command unmounts it.

.. note::

   There are more workshop CLI commands;
   some have a :option:`!--project` option
   that accepts a pathname to use as the project directory.

External changes to the project are tracked by the |project_markup| daemon.
Thus, if the project moved or copied,
all workshops referencing it are updated
so you can continue working uninterrupted.

If the project is deleted by external means,
workshops still referencing it
enter the *Error* state;
the only applicable command will be :command:`workshop remove`.


See also
--------

Explanation:

- :ref:`exp_sdk`
- :ref:`exp_workshop`