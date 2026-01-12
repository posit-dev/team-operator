# Launcher Templates

It's sort of a long story... but we use `job.tpl` and `service.tpl` (see versioned folders below) to launch sessions
for Workbench and Connect via the Posit Job Launcher.

These template files allow for customization of the session launching process. They are versioned. Moreover, we have
extended the template files for the `helm` charts and our operator... hopefully these changes will make their way
upstream at some point... but until then, we must maintain the differences for each change.

We do this in order to allow sufficient customization by customers _without_ modifying the template files themselves.

For example, if you want to add annotations, labels, or an imagePullSecret, you can do so via a helm chart value or
configuration setting in our `Workbench.Config` struct. Instead of having to monkey with Go Templating.

## Helm Charts

This construction initially started in the helm charts.

[Check out the directory here](https://github.com/rstudio/helm/tree/main/examples/launcher-templates)
[And documentation on usage here](https://github.com/rstudio/helm/blob/main/docs/customize.md)

## Issues

There are a handful of issues throughout the Launcher repo, Helm repo, and Connect repo discussing these items. The
Connect team, in particular, is quite familiar with the whole stack.

## Updates

In order to process updates, we typically copy new versions of the Launcher templates into the [helm repo](https://github.com/rstudio/helm)
under the `examples/launcher-templates/default` directory.

Let's say the version is `3.0.0`. We will then diff the `3.0.0` template against the `2.4.0` template (or whatever was previous).
This gives us a view of what has changed between versions.

Then we go to the `examples/launcher-templates/helm` directory and copy the latest templates to the new version. 
i.e. `2.4.0-v1` to `3.0.0-v1`.

Then we diff the `3.0.0-v1` against the `3.0.0` chart and add the new components.

Yes. This is tedious.

Yes. It is prone to error.

We do our best... and fix any bugs that may come from the less-than-ideal process.

## Why?

Why do we do this? Well... customers need to be able to set many attributes on their session
jobs. `labels`, `annotations`, `imagePullSecrets`. You name it.

Many of these are _required_.

Some of these are set by administrators (i.e. Kubernetes admins) in order for jobs to function.

Others might be set by users or application administrators.

Ideally, the products would manage most of this work for us and no other customization would be required.

However, that ideal has yet to be realized. As a result, this is what we have.

We cover _most_ cases and handle the complexity for customers, with a very nice escape hatch for "about as advanced and
complex as you want" exposed for those willing to delve into the depths!
