FROM fedora

ADD quartermaster /usr/local/bin

# Define default command.
CMD ["/usr/local/bin/quartermaster"]
