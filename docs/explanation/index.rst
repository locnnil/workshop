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


.. _exp_arch:

Architecture
------------

The architecture section provides a detailed overview of Workshop's design,
its components
and how they work together to provide isolated development environments.

.. toctree::
   :titlesonly:
   :class: flat-toctree

   architecture/index


.. _exp_workshop:

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
   :titlesonly:
   :class: flat-toctree

   workshops/index


.. _exp_sdks:

SDKs
----

.. @artefact SDK

SDKs are packages of software dependencies that can be installed in workshops
to create tailored development environments.

.. toctree::
   :titlesonly:
   :class: flat-toctree

   sdks/index


.. _exp_interfaces:

Interfaces
----------

Interfaces allow communication and resource sharing
between a workshop and the host system,
as well as between the different SDKs that are part of a workshop.

.. toctree::
   :titlesonly:
   :class: flat-toctree

   interfaces/index


Security considerations
-----------------------

This overview discusses the security aspects of |ws_markup| and |sdk_markup|,
such as isolation, privileges, relevant risks, and interface mechanics.
See :ref:`Security policy <security>`.
