// sub_141491BE0  va=0x141491BE0  size=293

void __fastcall sub_141491BE0(__int64 a1)
{
  __int64 v2; // rax
  _LocaleUpdate *v3; // rdi
  _LocaleUpdate *v4; // rcx
  __int64 v5; // rax
  __int64 v6; // [rsp+38h] [rbp-40h]
  _QWORD v7[4]; // [rsp+40h] [rbp-38h] BYREF

  if ( *(_QWORD *)(a1 + 8) != 0 && ((__int64 (*)(void))sub_14501B3E0)() != 0 )
  {
    v2 = sub_14501B3E0(*(_QWORD *)(a1 + 8));
    v3 = nullptr;
    if ( _RTDynamicCast(v2, 0, &off_14DCB4FF0, &off_14DCB64C8, 0) != 0 )
    {
      *(_DWORD *)((char *)&v7[1] + 6) = -1;
      *(_DWORD *)((char *)&v7[2] + 5) = -1;
      BYTE5(v7[1]) = 1;
      BYTE2(v7[2]) = *(_BYTE *)(a1 + 56);
      *(_WORD *)((char *)&v7[2] + 3) = *(_WORD *)(a1 + 60);
      v4 = qword_14E6387A8;
      if ( qword_14E6387A8 == nullptr )
      {
        v5 = sub_146E8BA20(176);
        v6 = v5;
        __wind
        {
          if ( v5 != 0 )
            v3 = (_LocaleUpdate *)sub_140B896A0(v5);
        }
        __unwind
        {
          j_j_scalable_free(v6, 176);
        }
        qword_14E6387A8 = v3;
        (**(void (__fastcall ***)(_LocaleUpdate *))v3)(v3);
        v4 = qword_14E6387A8;
      }
      sub_140B8AD40((__int64)v4, (__int64)v7);
      qmemcpy((void *)(a1 + 1104), v7, 25);
    }
  }
}
