;; Require necessary packages
(require 'sb-cover)
;; Load entire test system
(ql:quickload :acc/test)
;; Recompile just the acc system with coverage flag
(declaim (optimize sb-cover:store-coverage-data))
(asdf:compile-system :acc :force t)
(asdf:load-system :acc)
(declaim (optimize (sb-cover:store-coverage-data 0)))
;; Run all tests
(unless (fiveam:run-all-tests)
  (with-open-file
      (stream "shields.txt" :direction :output :if-does-not-exist :create)
    (format stream "https://img.shields.io/badge/coverage-fail-red"))
  (sb-ext:quit :unix-status 1))
;; Calculate expression coverage
(defparameter
  coverage-value
  (loop
 with exp-count = 0
 with cov-count = 0
 for file in (sb-cover:save-coverage)
   when (cl-ppcre:scan "src/.+/.*\\.lisp$" (car file))
 do (loop for expr in (cdr file)
            when (typep (caar expr) 'integer)
          do (progn
              (incf exp-count)
              (when (eq (cdr expr) t) (incf cov-count))))
 finally (return (* 100.0 (/ cov-count exp-count)))))
;; Save shields URL to file
(with-open-file (stream "shields.txt" :direction :output :if-does-not-exist :create)
  (format
      stream
      "https://img.shields.io/badge/coverage-~D%25-~A"
    (round coverage-value)
    (cond
     ((>= coverage-value 90.0) "green")
     ((>= coverage-value 70.0) "yellow")
     (t "red"))))
