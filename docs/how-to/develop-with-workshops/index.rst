.. _how_develop_workshops:

.. meta::
   :description: How-to guides on usage of workshops
                 with well-known developer tools.

How to develop with workshops
=============================

These articles cover the aspects of using |ws_markup| with developer tooling,
from connecting your favourite IDE to integrating with version control and CI/CD.


Connect an IDE
--------------

You can connect a locally installed IDE to a workshop over SSH,
which gives you full access to your workshop's filesystem and tools
from your existing editor setup:

.. toctree::
   :maxdepth: 1

   Connect VS Code to a workshop <connect-vscode>
   Run JetBrains Gateway in a workshop <run-jetbrains-gateway>


Run tools in the browser
------------------------

If you prefer a browser-based workflow or don't have a local IDE installed,
you can run an editor or notebook environment directly inside your workshop
and access it in your browser:

.. toctree::
   :maxdepth: 1

   Run VS Code in your browser <run-vscode-in-browser>
   Run JupyterLab in your browser <run-jupyterlab-in-browser>


Integrate with development workflows
------------------------------------

These guides cover using workshops alongside
version control, CI/CD, and AI-powered development tools:

.. toctree::
   :maxdepth: 1

   Use workshops with Git <use-git>
   Run GitHub Actions locally <run-github-actions-locally>
   Use workshops with AI agents <use-workshops-with-ai-agents>
