.. _how_git_workshops:

How to use workshops with Git
=============================

Workshops are intended for use in common development ecosystems,
which makes running into Git almost inevitable.
Let's see how you can integrate workshops into your repo.


Initialisation
--------------

To start, place the :ref:`workshop definition <exp_workshop_def>`
in your repository:

.. code:: console

   $ git init original
   $ cd original/


.. code-block:: yaml
   :caption: .workshop.golang.yaml

   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable


Next,
:ref:`launch <ref_workshop_launch>` the workshop
and start working on your code:

.. code:: console

   $ workshop launch golang


.. code-block:: go
   :caption: main.go

   package main

   import "fmt"

   func main() {
       fmt.Println("hello, Workshop")
   }


Mind that any activities
that rely on the workshop's contents
should now occur inside the workshop:

.. code:: console

   $ workshop exec golang -- go build -x main.go


However, the resulting artefacts are exposed in the project directory:

.. code:: console

   $ ./main

       hello, Workshop


They stay there even if you remove the workshop:

.. code:: console

   $ workshop remove golang
   $ ./main

       hello, Workshop


.. tip::

   If you actually remove the workshop at this step of the guide,
   relaunch it before proceeding further:

   .. code:: console

      $ workshop launch golang


From here, do with your repo as you please
because |project| handles
:ref:`moving files around <exp_moving_projects>`
quite well.

With your dependencies factored out like this,
recovering your build system after cloning the repo elsewhere
comes down to re-launching the workshop from a new
:ref:`project directory <exp_project>`.

But what if you have to maintain several branches
that require different versions of the same workshop?
A common solution is to clone the repo several times
and synchronise it manually when needed,
but this approach is prone to errors and overhead.
Let's build something better by...


Using worktrees
---------------

Let's now use a feature of Git that overlaps nicely with workshops,
namely :literalref:`git worktree<https://git-scm.com/docs/git-worktree>`.

One of |project|'s goals is
to simplify toggling external dependencies
such as frameworks or system versions.
Suppose you want to investigate an issue that appears on an older OS version,
so you create a new worktree just for that:

.. code:: console

   $ git worktree add ../hotfix/
   $ cd ../hotfix/


Instead of troubling yourself with virtual machines,
amend the definition
to change the :ref:`base image <exp_workshop_base>`:

.. code-block:: yaml
   :caption: .workshop.golang.yaml
   :emphasize-lines: 2

   name: golang
   base: ubuntu@20.04
   sdks:
     go:
       channel: latest/stable


Next, you launch the redefined workshop to work :

.. code:: console

   $ workshop launch golang
   $ # Hacking away until the problem is solved
   $ git commit -m "solve problem with hotfix"
   $ cd ../original
   $ git merge hotfix


Just like :ref:`with regular directories <exp_moving_projects>`,
|project| cooperates nicely with
:literalref:`git worktree move<https://git-scm.com/docs/git-worktree#_commands>`:

.. code:: console

   $ git worktree move ../hotfix/ ../resolved/


Accordingly,
when it comes to clean-up,
remove all workshops
before running :samp:`git worktree remove`:

.. code:: console

   $ cd ../resolved/
   $ workshop remove golang
   $ cd ../original/
   $ git worktree remove ../resolved/


Thus, using :command:`git worktree` reduces the effort to sync, stash and pull,
while |project| enables you to hot-swap an entire OS
or other complex dependencies
by changing from one directory to another.