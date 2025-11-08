(in-package :acc)

(fiveam:def-suite instruction)

(test operand-test-generic
  (is (string= "UNDEFINED" (to-string (make-instance 'operand)))))

(def-fixture instruction-test-env ()
  (let ((generic-op (make-instance 'operand)))
    (&body)))

(test instruction-test
  (with-fixture instruction-test-env ()
    (is (string=
          "op"
          (to-string (make-instance 'instruction :op "op" :indent nil))))
    (is (string=
          "op UNDEFINED"
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op) :indent nil))))
    (is (string=
          "op UNDEFINED, UNDEFINED"
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op generic-op) :indent nil))))
    (is (string=
          (format nil "~cop" #\Tab)
          (to-string (make-instance 'instruction :op "op" :indent t))))))