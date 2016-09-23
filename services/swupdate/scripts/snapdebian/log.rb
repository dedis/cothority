require 'logger'
$logger = Logger.new(STDOUT)
#$logger.datetime_format = '%Y-%m-%d %H:%M:%S'
#$logger.datetime_format = '%H:%M:%S'
$logger.formatter = proc do |severity, datetime, progname, msg|
  "#{datetime.strftime("%H:%M:%S")} #{severity} #{Thread.current[:name]}: #{msg}\n"
end
$logger.level = Logger::INFO
Thread.current[:name] = "main"


