;; Load necessary packages
(require 'asdf)
(require 'sb-cover)
(ql:quickload :fiveam)
(ql:quickload :cl-ppcre)
;; Load acc w/ coverage data compiler flag
(declaim (optimize sb-cover:store-coverage-data))
(asdf:oos 'asdf:load-op :acc :force t)
(declaim (optimize (sb-cover:store-coverage-data 0)))
;; Run all tests
(handler-case (fiveam:run-all-tests)
  (condition (c)
             (declare (ignore c))
             (with-open-file
                 (stream "shields.txt"
                         :direction :output
                         :if-exists :supersede
                         :if-does-not-exist :create)
               (format stream "https://img.shields.io/badge/coverage-fail-red"))
             (sb-ext:quit)))
;; Calculate expression coverage
(defvar
  cov-perc
  (loop
 with exp-count = 0
 with cov-count = 0
 for file in (sb-cover:save-coverage)
   when (cl-ppcre:scan "src/.*\\.lisp$" (car file))
 do (loop for expr in (cdr file)
            when (typep (first (car expr)) 'integer)
          do (progn
              (incf exp-count)
              (when (eq (cdr expr) t) (incf cov-count))))
 finally (progn
  (format t "~D out of ~D expressions covered~%" cov-count exp-count)
  (return (* 100.0 (/ cov-count exp-count))))))
;; Save shields URL to file
(with-open-file (stream "shields.txt"
                        :direction :output
                        :if-exists :supersede
                        :if-does-not-exist :create)
  (format
      stream
      "https://img.shields.io/badge/coverage-~D%25-~A"
    (round cov-perc)
    (cond
     ((>= cov-perc 90.0) "green")
     ((>= cov-perc 70.0) "yellow")
     (t "red"))))
;; Done
(sb-ext:quit)
