// Copyright 2017, Collabora Ltd.

/*
Package 'actions' implements 'debos' modules used for OS creation.

The origin property

Several actions have the 'origin' property. Possible values for the
'origin' property are:

  1) 'recipe' ....... directory the recipe is in
  2) 'filesystem' ... target filesystem root directory from previous filesystem-deploy action or
                      a previous ostree action.
  3) name property of a previous download action

*/
package actions
