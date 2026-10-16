module App
  DEFAULT_TIMEOUT = 30

  module Helpers
    def self.format(val)
      val.to_s
    end
  end

  class Client
    MAX_RETRIES = 3

    def initialize(host)
      @host = host
    end

    def fetch(path)
      msg = Helpers.format(path)
      msg
    end

    def self.create(host)
      new(host)
    end
  end
end

def top_helper(x)
  x
end
