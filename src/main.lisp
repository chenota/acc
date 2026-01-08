(in-package :acc)

(opts:define-opts
  (:name :help
         :description "Help text"
         :short #\h
         :long "help"))

(defmacro with-report-error (category &body form)
  `(handler-case (progn ,@form)
     (t (c)
        (format *error-output* "~A Error: ~A~%" ,category c)
        (opts:exit 1))))

(defun slurp-input (path)
  "Read stdin or a file to a string"
  (if (or (string= path "-") (null path))
      (uiop:slurp-input-stream 'string *standard-input*)
      (with-open-file (stream path
                              :direction :input
                              :if-does-not-exist :error)
        (uiop:slurp-input-stream 'string stream))))

(defun write-output (instructions path)
  "Write a list of instructions to a file"
  (flet ((do-write
          (stream)
          (dolist (instruction instructions)
            (write-line (to-string instruction) stream))))
    (if (or (null path) (string= path "-"))
        (do-write *standard-output*)
        (uiop:with-output-file
          (stream path :if-exists :supersede)
          (do-write stream)))))

(defun compile-string-to-instructions (source)
  "Compile an input file into an output file"
  (let* ((token-list (tokenize source))
         (token-sequence (make-token-sequence token-list))
         (ast (parse-program token-sequence)))
    (gen-program ast)))

(defun main ()
  (multiple-value-bind (options free-args)
      ;; Read options
    (handler-case (opts:get-opts)
      ;; Handle unknown option
      (opts:unknown-option (c)
                           (format *error-output* "Unknown option: ~s~%" (opts:option c))
                           (opts:exit 1)))
    ;; Handle help flag
    (when (getf options :help)
          (opts:describe :prefix "Usage: acc [options] <INPUT-FILE> <OUTPUT-FILE>")
          (opts:exit 0))
    ;; Check that there's two free args
    (unless (= 2 (length free-args))
      (format *error-output* "Invalid arguments: Expected two positional arguments~%")
      (opts:exit 1))
    (let* ((source (with-report-error "IO" (slurp-input (first free-args))))
           (token-list (with-report-error "Lexical" (tokenize source)))
           (token-sequence (make-token-sequence token-list))
           (ast (with-report-error "Syntax" (parse-program token-sequence)))
           (instructions (with-report-error "Generation" (gen-program ast))))
      (with-report-error "IO" (write-output instructions (second free-args))))))