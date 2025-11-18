(in-package :acc)

(fiveam:def-suite instruction)

(test string-operand-test
  (is (string= "\"VALUE\"" (to-string (make-string-operand "VALUE")))))

(test ident-operand-test
  (is (string= "VALUE" (to-string (make-ident-operand "VALUE")))))

(test type-operand-test
  (is (string= "@VALUE" (to-string (make-type-operand "VALUE")))))

(def-fixture instruction-test-env ()
  (let ((generic-op (make-ident-operand "VALUE")))
    (&body)))

(test instruction-test
  (with-fixture instruction-test-env ()
    (is (string=
          "op"
          (to-string (make-instance 'instruction :op "op" :indent nil))))
    (is (string=
          "op VALUE"
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op) :indent nil))))
    (is (string=
          "op VALUE, VALUE"
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op generic-op) :indent nil))))
    (is (string=
          (format nil "~cop" #\Tab)
          (to-string (make-instance 'instruction :op "op" :indent t))))))