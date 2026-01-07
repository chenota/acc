(in-package :acc)

(opts:define-opts
  (:name :help
         :description "Help text"
         :short #\h
         :long "help"))

(defun main ()
  (multiple-value-bind (options free-args)
      ;; Read options
    (handler-case (opts:get-opts)
      ;; Handle unknown option
      (opts:unknown-option (c)
                           (format *error-output* "Unknown option: ~s~%" (opts:option c))
                           (opts:exit 1)))
    (declare (ignore free-args))
    ;; Handle help flag
    (when (getf options :help)
          (opts:describe :prefix "Usage: acc [options] [FILE]")
          (opts:exit 0))
    ;; Main logic
    (format t "Hello, world!~%")))