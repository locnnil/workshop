:hide-toc:

Project, workshop, SDKs
=======================

.. toctree::
   :hidden:
   :maxdepth: 1

   project
   sdks
   workshop


Projects, workshops and their SDKs
are the key building blocks of |project_markup|.
To start using |project_markup|,
it is important to understand how these concepts fit together.

You can view a *project* as your working directory,
doing all the things you would usually do there:
create and populate repositories, write and build code, run models, and so on.
However, the difference starts with the software dependencies
you would earlier install as system-wide packages, container images,
or in myriad other ways.
Instead, they are wrapped and published as |project_markup|-ready, isolated *SDKs*
which you list while defining a *workshop*.

A single workshop always points to a project,
and a project may have multiple workshops referencing it,
with each workshop containing a number of SDKs.

What do you get out of this multitude?

First, |project_markup| is transactional in nature;
you won't have to trace residual files and libraries all across your system
after you uninstall a package that turned out too unstable to your taste.
Even if an SDK dumps something unexpected onto the disk,
it's contained within the workshop.
|project_markup| aims to encapsulate each part of functionality you may need,
keeping things clean and tidy.

Next, it's portable;
imagine sending a *compact* tarball of your project to a coworker
who then recreates it with exactly the same dependency versions that you used.
What's better, this is achieved without manually customised,
high-maintenance image definitions or configurations;
all the work of keeping the SDKs in your workshop operational
is done by the people who are actually responsible for it
(namely, the publishers).


See also
--------

Explanation:

- :ref:`project (concept) <exp_project>`
- :ref:`SDKs (concept) <exp_sdk>`
- :ref:`workshop (concept) <exp_workshop>`