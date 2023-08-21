:relatedlinks: [Diátaxis](https://diataxis.fr/)

.. _home:

|project|
=========

**|project| is a tool that automates intricate prerequisite setup
for your projects**.

**Define your dev environment in straightforward YAML**.
The tool consumes the definition to create a contained workspace,
installs the dependencies it lists as a number of SDKs,
and attaches their life cycle hooks for run-time control.
IDEs such as Visual Studio Code or JupyterLab
can discover workspaces and leverage them in their operation,
tidying up your system and streamlining your work.

**Untangle the know-how that was weaved into your project**.
An environment that could take hours of setup
can be launched with one command;
workspaces enhance issue reproduction across platforms,
facilitate collaboration in code reviews,
and confine hackish experiments in lightweight containers.

**Mitigate your setup's complexity with |project|.**
AI/ML, robotics, IoT, EdTech, and similar domains
commonly have less-than-trivial project layouts
that depend on multiple Linux distributions,
a plethora of SDKs from different publishers,
and a grocery list of libraries and programming languages.

---------


In this documentation
---------------------

.. grid:: 1 1 2 2

   .. grid-item:: :doc:`Tutorial <tutorial/index>`

      **Start here**: a hands-on introduction to |project| for new users


   .. grid-item:: :doc:`Explanation <explanation/index>`

      **Discussion and clarification** of key topics

---------


.. toctree::
   :hidden:
   :maxdepth: 2

   tutorial/index
   explanation/index
   ReadMe <README>
