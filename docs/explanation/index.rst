.. _exp_index:

.. meta::
   :description: An overview of Workshop's core concepts, explaining how
                 workshops, projects, and SDKs work together to create
                 consistent development environments and help build workflows.

Explanation
===========

These explanatory articles cover the main building blocks of |ws_markup|.
To start using |ws_markup| and |sdk_markup|,
it is important to understand how these concepts fit together.


Architecture
------------

The architecture section provides a detailed overview of Workshop's design,
its components
and how they work together to provide isolated development environments.

.. toctree::
   :hidden:
   :titlesonly:

   architecture/index


- :doc:`architecture/installation`


Workshops and projects
----------------------

.. @artefact project
.. @artefact workshop (container)
.. @artefact workshop definition

Workshops are development environments, each running in a container,
mapping your project to its contained dependencies.
In turn, a project is a working directory
where multiple workshop definitions can be placed.

.. toctree::
   :hidden:

   workshops/index

- :doc:`workshops/concepts`
- :doc:`workshops/changes-tasks`
- :doc:`workshops/projects`
- :doc:`workshops/workshop-cli`


SDKs
----

.. @artefact SDK

SDKs are packages of software dependencies that can be installed in workshops
to create tailored development environments.

.. toctree::
   :hidden:

   sdks/index


- :doc:`sdks/concepts`
- :doc:`Data storage and sharing <sdks/data-persistence-sharing>`
- :doc:`SDKs versus Dockerfiles <sdks/sdk-vs-dockerfile>`
- :doc:`Parts <sdks/parts>`


Interfaces
----------

Interfaces allow communication and resource sharing
between a workshop and the host system,
as well as between the different SDKs that are part of a workshop.

.. toctree::
   :hidden:

   interfaces/index

- :doc:`interfaces/concepts`
- :doc:`interfaces/camera-interface`
- :doc:`interfaces/desktop-interface`
- :doc:`interfaces/gpu-interface`
- :doc:`interfaces/mount-interface`
- :doc:`interfaces/ssh-interface`
- :doc:`interfaces/tunnel-interface`


Security considerations
-----------------------

This overview discusses the security aspects of |ws_markup| and |sdk_markup|,
such as isolation, privileges, relevant risks, and interface mechanics.

.. toctree::
   :maxdepth: 1

   Security <../security>
