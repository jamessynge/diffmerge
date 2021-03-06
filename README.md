# diffmerge
Experiments with creating an alternate to diff3 and merge, motivated
by my annoyance with these issues:

1) Ordered lists are not auto-merged. For example, build dependencies 
   and C++ include lists are typically in lexically sorted,
   so the merge tool should check to see if conflicting lines are
   part of such a list in all 3 files, and if so treat them as
   ordered sets, where
   we don't care about the conflicting lines, but rather about the
   set operations performed: insert and remove; and those operations
   won't be in conflict.

2) Mis-alignment of matches for added functions. For example, we  desire
   this, where are the lines of the new function have been marked new:
```
     $diff -y file2 file1
     void func1() {            void func1() {
       x += 1                     x += 1
     }                         }

                             > void functhreehalves() {
                             >   x += 1.5
                             > }
                             >
     void func2() {            void func2() {
       x += 2                    x += 2
     }                         }
```
   However, we often get this from diff, and hence from merge:
```
     $diff -y file2 file1
     void func1() {            void func1() {
       x += 1                     x += 1
     }                       > }
                             >
                             > void functhreehalves() {
                             >   x += 1.5
                               }

     void func2() {            void func2() {
       x += 2                    x += 2
     }                         }
```
   (example from http://fabiensanglard.net/git_code_review/diff.php).

3) Non-conflicting intra-line changes are not recognized as such; for example:
```
     Base                 |   Yours                |   Theirs
     if (y == x + 19) {   |   if (y == x + 17) {   |   if (z == x + 19) {
                          |                ^^      |       ^
```
   merge considers the lines in conflict:
```
     <<<<<<< Yours
     if (oldVar == x + 17) {
     =======
     if (newVar == x + 19) {
     >>>>>>> Theirs
```
   It isn't clear where the limit is on how close such changes can be before
   we must not auto-merge. What would be nice is to have a UI which makes it
   easy to suggest changes, yet still give the user an easy way to make their
   own choices, perhaps spanning multiple conflict points.

4) Moves of a block of lines should not be considered adds of new lines and
   deletes of old lines, but rather recognized as a move so that changes
   within the block of lines can be automatically merged.

5) If you've renamed a symbol in your file (i.e. changed ALL lines that
   contain that symbol), and the other file adds uses of that symbol,
   the tool should offer some means of automatically applying the renaming
   to the new uses.  BONUS.

6) Changing the indentation of lines is treated as changing the entire line,
   rather than being treated as separate from changing the characters to the
   right of the indentation.
