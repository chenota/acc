(in-package :acc)

(with-ignore-coverage
  (defstruct assign-type-environment
    (function-return-context nil))
  (defgeneric assign-type (node env)
    (:documentation "Assigns and returns the type of the node within the given environment")))

(defun set-program-types (ast)
  (assert (program-node-p ast))
  (assign-type ast (make-env))
  ast)

(defmethod assign-type ((node program-node) env)
  (loop for func in (program-node-functions node) do (assign-type func env))
  nil)

(defmethod assign-type ((node function-node) env)
  (let ((inner-env (env-extend env)))
    (setf (env-return-type inner-env) (function-node-return-type node))
    (loop for stmt in (function-node-body node) do (assign-type stmt inner-env))
    (setf
      (function-node-type-info node)
      (make-function-type :parameters nil :return-type (function-node-return-type node)))))

(defmethod assign-type ((node return-statement-node) env)
  (let ((t1 (assign-type (return-statement-node-expression node) env)))
    (unless
        (valid-cast-p t1 (find-return-type env))
      (error 'location-error
        :location (ast-node-location node)
        :message (format nil "Invalid return type: ~A" t1)))
    nil))

(defmethod assign-type ((node cast-node) env)
  (let ((t1 (assign-type (cast-node-expression node) env)))
    (unless (valid-cast-p t1 (cast-node-cast-type node))
      (error
          'location-error
        :location (ast-node-location node)
        :message (format nil "Invalid type cast: ~A to ~A" t1 (cast-node-cast-type node))))
    (setf (cast-node-type-info node) (cast-node-cast-type node))))

(defmethod assign-type ((node int-node) env)
  (setf (int-node-type-info node) (make-integer-type :size :generic)))

(defun valid-cast-p (source-type destination-type)
  (not (or (null source-type) (null destination-type))))