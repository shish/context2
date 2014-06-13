              _________                __                   __   
              \_   ___ \  ____   _____/  |_  ____ ___  ____/  |_ 
              /    \  \/ /  _ \ /    \   __\/ __ \\  \/  /\   __\
              \     \___(  <_> )   |  \  | \  ___/ >    <  |  |  
               \______  /\____/|___|  /__|  \___  >__/\_ \ |__|  
                      \/            \/          \/      \/       


If you need any help, contact webmaster@shishnet.org


Creating Data
~~~~~~~~~~~~~
Context on its own doesn't do much, you need to give it some data to analyse;
to generate data, you need to get your code to log important points -- for a
tutorial / example, check the website at 

  - http://code.shishnet.org/context

Pre-written libraries for various languages are in a separate APIs project
available at

  - https://github.com/shish/context-apis


Compiling (Optional)
~~~~~~~~~~~~~~~~~~~~
The APIs generate a simple linear .ctxt file with all sorts of information
inter-woven, written out as it happens. For more efficient data access, the
.ctxt needs to be compiled into a .cbin -- you can either do this manually,
with context-compiler; or context-viewer will do it when loading a file.


Viewing
~~~~~~~
Run `context-viewer [filename]`, where the filename is either a .ctxt or a
.cbin file.
