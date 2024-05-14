Workshops, SDKs
===============

These articles explain the core set of |project_markup|-related concepts.

.. toctree::
   :glob:
   :maxdepth: 1

   *


Summary
-------

Projects, workshops and SDKs
are the key building blocks of |project_markup|.
To start using |project_markup|,
it is important to understand how these concepts fit together.

You can view a *project* as your working directory,
doing all the things you would usually do there:
create and populate repositories, write and build code, run models, and so on.
However, the difference starts with the software dependencies
you would previously install as system-wide packages, container images,
or in myriad other ways.

Instead, they are packed and published as |project_markup|-ready, isolated *SDKs*
that you list while defining a *workshop*.
In turn, it is a container that is launched according to the workshop definition,
which resides in a :file:`.yaml` file in the project directory.

To address a few points of confusion straight away:

- A lowercase *workshop* is a container tied to a definition and a project;
  it can be plural and shouldn't be confused with |project_markup| itself.

- Two identical workshop definitions in two separate projects
  result in two different workshops.

- Two workshops in the same project share the project directory,
  mapped inside both workshops as :file:`/project/`.

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
imagine sending a *compact* snapshot of your project environment to a coworker
who then recreates it with exactly the same dependency versions that you used.
What's better, this is achieved without manually customised,
high-maintenance image definitions or configurations;
all the work of keeping the SDKs in your workshop operational
is done by the people who are actually responsible for it
(namely, the publishers).
