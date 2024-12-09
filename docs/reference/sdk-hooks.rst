:hide-toc:

.. _ref_sdk_hooks:

SDK hooks
=========

|ws_markup| supports the following life cycle hooks:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 2 5 4

   * - Name
     - When it runs
     - What it does

   * - :samp:`check-health`
     - At :ref:`ref_workshop_launch`:
       after running :samp:`setup-base` hooks for *all* SDKs.
     
       At :ref:`ref_workshop_refresh`:
       after running :samp:`restore-state` hooks for *all* SDKs.

     - Reports the health of the SDK
       (*okay*, *waiting* or *error*)
       so |ws_markup|
       can determine the overall state of the workshop.


   * - :samp:`restore-state`

     - At :ref:`ref_workshop_refresh`:
       after launching the new workshop
       and running the :samp:`setup-base` hook for the SDK.

     - Restores SDK-specific data,
       collectively referred to as *state*,
       from a |ws_markup|-defined location.
       The hook itself comes from the *new* SDK version.


   * - :samp:`save-state`

     - At :ref:`ref_workshop_refresh`:
       before destroying the old workshop.

     - Preserves SDK-specific data,
       collectively referred to as *state*,
       at a |ws_markup|-defined location.
       The hook itself comes from the *old* SDK version.


   * - :samp:`setup-base`

     - At :ref:`ref_workshop_launch`, :ref:`ref_workshop_refresh`:
       after unpacking the base image
       and starting the workshop,
       but before setting its status to *Ready*.

     - Configures the base image for the SDK to become operational.


Hooks of the same type from multiple SDKs run in a non-deterministic sequence.
You should not rely on any particular order of their execution.


See also
--------

Explanation:

- :ref:`exp_sdk`
- :ref:`exp_sdk_state`
- :ref:`exp_base`
- :ref:`exp_workshop_def`
