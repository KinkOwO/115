// singleton_user_sub_1444A34E0

char __fastcall sub_1444A34E0(_QWORD *a1)
{
  __int64 v2; // rcx
  int v3; // eax
  __int64 v4; // rdi
  __int64 v5; // rcx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  _BYTE *v8; // rax
  _BYTE v10[32]; // [rsp+48h] [rbp-20h] BYREF

  sub_145F6F4B0(a1);
  v2 = a1[33];
  if ( v2 )
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v2 + 16LL))(v2, 0);
  (*(void (__fastcall **)(_QWORD *, _QWORD))(*a1 + 400LL))(a1, 0);
  v3 = sub_146E8C7D0(&unk_1497CB8F0);
  sub_145A31380(v3, -1, 0, 0, -1, -1, 0);
  v4 = a1[194];
  v5 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v6 = sub_146E8BA20(496);
    if ( v6 )
      v7 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v6);
    else
      v7 = 0;
    qword_14E659EA8 = (__int64)v7;
    (**v7)(v7);
    v5 = qword_14E659EA8;
  }
  v8 = (_BYTE *)sub_14449E0B0(v5, v10);
  sub_146EECBB0(v4, 24 - (unsigned int)(*v8 != 0));
  sub_1467A9F20(a1, 0);
  return 1;
}

