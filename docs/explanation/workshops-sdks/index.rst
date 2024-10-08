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
are the main building blocks of |project_markup|.
To start using |project_markup|,
it is important to understand how these concepts fit together.

You can think of a *project* as your working directory,
and do everything you would usually do there:
create and populate repositories, write and build code, run models, and so on.
The difference starts with the software dependencies
that you used to install as system-wide packages, container images,
or in myriad other ways.

Instead, they are packed and published as |project_markup|-ready,
isolated *SDKs* that you list while you define a *workshop*.
The workshop definition is a :file:`.yaml` file in the :file:`.workshop`
directory, and the workshop itself is the container built according to this
definition.

To clear up a few points of confusion straight away:

- A *workshop* is a container tied to a definition and a project;
  not to be confused with |project_markup| itself.

- Launching an identical definition from two different projects
  creates two separate workshops.

- Two workshops in a project share the same project directory,
  mapped inside both as :file:`/project/`.

A single workshop always points to a project,
and a project can have multiple workshops pointing to it,
with each workshop containing multiple SDKs.
What do *you* get out of this multiplicity, though?

Firstly, |project_markup| is transactional in nature;
you don't have to track down leftover files and libraries all over your system
after you've uninstalled a package that turned out to be too unstable.
Even if an SDK drops something unexpected on your drive,
it's contained within the workshop.
|project_markup| aims to encapsulate every piece of functionality you need,
keeping things clean and tidy.

Next, it's portable;
imagine sending a compact definition of your project environment to a colleague,
who can rebuild it with exactly the same dependency versions you used.
Better still, it's done without any manual, high-maintenance image definitions
or configurations;
all the work of keeping the SDKs in your workshop operational is done
by the people who are actually responsible for them: the publishers.
