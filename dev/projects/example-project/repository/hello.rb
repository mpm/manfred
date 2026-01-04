#!/usr/bin/env ruby
# frozen_string_literal: true

# A simple Ruby script for testing MANFRED
# This can be modified by Claude Code during job execution

def greet(name)
  "Hello, #{name}!"
end

def main
  puts greet("World")
  puts "The current time is: #{Time.now}"
end

main if __FILE__ == $PROGRAM_NAME
