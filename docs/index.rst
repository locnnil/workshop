:slug: home-page
:relatedlinks: [SDKcraft](https://canonical-sdkcraft.readthedocs-hosted.com/), [LXD](https://documentation.ubuntu.com/lxd/), [Snap](https://snapcraft.io/docs/)

.. _home:

|project_markup|
================

.. toctree::
   :hidden:

   Home <self>
   tutorial/index
   how-to/index
   explanation/index
   reference/index
   Contribution <contributing>


**A tool for defining and managing ephemeral development environments**.

**Define your prerequisites and dependencies in simple YAML**.
|project_markup| consumes the definition to create a contained workshop,
installs the components as a set of SDKs
and attaches custom actions for run-time control.
IDEs such as Visual Studio Code or JupyterLab can discover workshops
and use them in day-to-day operations,
tidying up your system and streamlining your work.

**Focus on your project, not your setup**.
An environment that could take hours to configure
can now be launched with a single command.
|project_markup| improves cross-platform issue reproduction,
preserves context in discussions or reviews
and confines bold experiments to transparent sandboxes.

**For those who build and maintain complex, error-prone workspaces**.
AI/ML, robotics, IoT, EdTech and similar domains
typically use less-than-trivial project layouts
that depend on multiple Linux distributions or images,
a plethora of SDKs from many vendors
and a grocery list of libraries and languages.
That's where |project_markup| thrives.

----


In this documentation
---------------------

.. grid:: 1 1 2 2

   .. grid-item:: :doc:`Tutorial <tutorial/index>`

      **Starter instructions** for new users of |project_markup|


   .. grid-item:: :doc:`How-to guides <how-to/index>`

      **Step-by-step guides** covering common tasks


   .. grid-item:: :doc:`Explanation <explanation/index>`

      **Discussion and clarification** of key topics


   .. grid-item:: :doc:`Reference <reference/index>`

      **Technical details**, specifications, APIs

----


Project and community
---------------------

|project_markup| is an emergent project
within the Enterprise Engineering department here at Canonical;
`SDKcraft`_
is a sibling project,
aimed at publishers who create and distribute SDKs for |project_markup|.

At its core, |project_markup| relies on
`LXD <https://documentation.ubuntu.com/lxd/>`_
to handle the low-level details that make the magic happen;
it also follows the tooling paradigm exemplified by
`Snap <https://snapcraft.io/docs/>`_.

Talk to us if you have a project in AI/ML, robotics or any other field
where setting up an environment is a daily or weekly activity
that can easily shave hours off your schedule.
Tell us about the frustrating parts of your experience,
and we'll see what |project_markup| can do for you.
Let us know if you have an SDK or framework to try with |project_markup|:
we'll help you get it out there.


- `Code of conduct <https://ubuntu.com/community/ethos/code-of-conduct>`__

- `Pulse reviews on Discourse <https://discourse.canonical.com/c/engineering/sdk/34>`__

- `Mattermost channel <https://chat.canonical.com/canonical/channels/sdk>`__

- `Product map <https://warthogs.atlassian.net/jira/software/c/projects/WSP/boards/1645>`__

- `Contribution and participation <https://github.com/canonical/workshop/pulls>`__

- `Product and documentation feedback <https://github.com/canonical/workshop/issues>`__
