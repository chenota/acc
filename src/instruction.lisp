(in-package :acc)

(defclass instruction ()
    ((indent :initarg :indent :initform nil)))

(defmethod print-instruction :before ((i instruction) s)
  "Check types and print tab indent if directed"
  (check-type s stream)
  (when (slot-value i 'indent) (format s "~c" #\Tab)))

(defmethod print-instruction :after ((i instruction) s)
  "Ensure instructions always end in a newline"
  (format s "~%"))

(defmethod print-instruction ((i instruction) s)
  (declare (ignore i))
  (format s "an instruction"))

(defclass file (instruction)
    ((name :accessor name :initarg :name :type string))
  (:default-initargs :indent t))

(defmethod print-instruction ((i file) s)
  (format s ".file ~s" (slot-value i 'name)))