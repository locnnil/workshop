Moving projects around
======================

It may be unclear how workshops respond to daily operations
such as moving or copying a project directory.
Let's spend some time talking about different aspects here.


Before launch
-------------

A workshop that you didn't :ref:`launch <ref_workshop_launch>`
is just that: a :ref:`definition file <exp_workshop_def>`
which behaves like any good file should.
Things start changing *after* you run :command:`workshop launch`:

.. code:: console

   $ cd /home/user/project/
   $ workshop launch golang

     "golang" launched


Moving a project
----------------

This is the most transparent scenario:

.. code:: console

   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/project      golang    Ready   -

   $cd ..
   $ mv /project/ PROJECT/

   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/PROJECT      golang    Ready   -


|project| handles the project's move seamlessly
so the workshop here stays operational like you would expect it to;
there are no loose ends to pick,
no paths to update in your definition file.

However, mind that |project| ensures only the workshop's safe transition,
so it's up to you to update the path in your project's metadata
that are not specific to |project|.


Copying a project
-----------------

Now let's copy the project directory instead:

.. code:: console

   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/PROJECT      golang    Ready   -

   $ cp -r /home/user/PROJECT/ /home/user/project/

   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/PROJECT      golang    Ready   -


|project| doesn't launch the workshop in the new directory,
which is probably the reasonable default here,
but what happens if you do it yourself?

.. code:: console

   $ cd project/
   $ workshop launch golang
   $ workshop list --global

       Project                 Workshop  Status  Notes
       /home/user/project      golang    Ready   -
       /home/user/PROJECT      golang    Ready   -


Now, these are two independent workshops that happen to have the same name,
not a single workshop that is somehow shared by multiple project directories.

Again, it's up to you to update the new project's metadata.


Removing a project
------------------

|project| doesn't handle deletion automatically;
make sure to remove all workshops
before deleting the project directory:

.. code:: console

   $ workshop remove golang
   $ cd ..
   $ rm -rf /home/user/PROJECT/