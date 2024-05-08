.. _how_moving_projects:

How to move projects around
===========================

It may be unclear how workshops respond to daily operations
such as moving or copying a project directory.
Let's spend some time talking about different aspects here.


Before launch
-------------

A workshop that you didn't :command:`launch`
is just that: a definition file
which behaves like any good file should.
Things start changing *after* you run :command:`workshop launch`:

.. code-block:: yaml
   :caption: /home/user/old/.workshop.golang.yaml

   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable


.. code-block:: console

   $ workshop launch golang --project /home/user/old/


Moving a project
----------------

This is the most basic scenario.
Start in the same project directory where you launched the workshop:

.. code-block:: console

   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/old          golang    Ready   -


Move the project directory and check the workshop:

.. code-block:: console

   $ mv /home/user/old/ /home/user/new/
   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/new          golang    Ready   -


|project_markup| handles the project's move seamlessly
so the workshop here stays operational like you would expect it to;
there are no loose ends to pick,
no paths to update in your definition file.

However, mind that |project_markup| ensures only the workshop's safe transition,
so it's up to you to update the path in your project's metadata
that are not specific to |project_markup|.


Copying a project
-----------------

Now let's copy the project directory.
Again, start with the directory where the workshop resides:

.. code-block:: console

   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/old          golang    Ready   -


Copy the project directory and check the workshops:

.. code-block:: console

   $ cp -r /home/user/old/ /home/user/new/

   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/old          golang    Ready   -


|project_markup| doesn't launch the workshop in the new directory,
which is probably the reasonable default here,
but what happens if you do it yourself?

.. code-block:: console

   $ workshop launch golang --project /home/user/new/
   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/old          golang    Ready   -
       /home/user/new          golang    Ready   -


Now, these are two independent workshops that happen to have the same name,
not a single workshop that is somehow shared by multiple project directories.

Again, it's up to you to update the new project's metadata.


Removing a project
------------------

|project_markup| doesn't handle deletion automatically;
make sure to remove all workshops
before deleting the project directory:

.. code-block:: console

   $ workshop remove golang --project /home/user/old/
   $ rm -rf /home/user/old/


See also
--------

Explanation:

- :ref:`exp_project`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_list`
- :ref:`ref_workshop_remove`
